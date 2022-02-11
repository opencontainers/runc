package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

// version must be set from the contents of VERSION file by go build's
// -X main.version= option in the Makefile.
var version = "unknown"

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile
var gitCommit = ""

const (
	specConfig = "config.json"
	usage      = `Open Container Initiative runtime

runc is a command line client for running applications packaged according to
the Open Container Initiative (OCI) format and is a compliant implementation of the
Open Container Initiative specification.

runc integrates well with existing process supervisors to provide a production
container runtime environment for applications. It can be used with your
existing process monitoring tools and the container will be spawned as a
direct child of the process supervisor.

Containers are configured using bundles. A bundle for a container is a directory
that includes a specification file named "` + specConfig + `" and a root filesystem.
The root filesystem contains the contents of the container.

To start a new instance of a container:

    # runc run [ -b bundle ] <container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host. Providing the bundle directory using "-b" is optional. The default
value for "bundle" is the current directory.`
)

func main() {
	app := cli.NewApp()
	app.Name = "runc"
	app.Usage = usage

	v := []string{version}

	if gitCommit != "" {
		v = append(v, "commit: "+gitCommit)
	}
	v = append(v, "spec: "+specs.Version)
	v = append(v, "go: "+runtime.Version())

	major, minor, micro := seccomp.Version()
	if major+minor+micro > 0 {
		v = append(v, fmt.Sprintf("libseccomp: %d.%d.%d", major, minor, micro))
	}
	app.Version = strings.Join(v, "\n")

	root := "/run/runc"
	xdgDirUsed := false
	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir != "" && shouldHonorXDGRuntimeDir() {
		root = xdgRuntimeDir + "/runc"
		xdgDirUsed = true
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug logging",
		},
		cli.StringFlag{
			Name:  "log",
			Value: "",
			Usage: "set the log file to write runc logs to (default is '/dev/stderr')",
		},
		cli.StringFlag{
			Name:  "log-format",
			Value: "text",
			Usage: "set the log format ('text' (default), or 'json')",
		},
		cli.StringFlag{
			Name:  "root",
			Value: root,
			Usage: "root directory for storage of container state (this should be located in tmpfs)",
		},
		cli.StringFlag{
			Name:   "criu",
			Usage:  "(obsoleted; do not use)",
			Hidden: true,
		},
		cli.BoolFlag{
			Name:  "systemd-cgroup",
			Usage: "enable systemd cgroup support, expects cgroupsPath to be of form \"slice:prefix:name\" for e.g. \"system.slice:runc:434234\"",
		},
		cli.StringFlag{
			Name:  "rootless",
			Value: "auto",
			Usage: "ignore cgroup permission errors ('true', 'false', or 'auto')",
		},
	}
	app.Commands = []cli.Command{
		checkpointCommand,
		createCommand,
		deleteCommand,
		eventsCommand,
		execCommand,
		killCommand,
		listCommand,
		pauseCommand,
		psCommand,
		restoreCommand,
		resumeCommand,
		runCommand,
		specCommand,
		startCommand,
		stateCommand,
		updateCommand,
		featuresCommand,
	}
	app.Before = func(context *cli.Context) error {
		if !context.IsSet("root") && xdgDirUsed {
			// According to the XDG specification, we need to set anything in
			// XDG_RUNTIME_DIR to have a sticky bit if we don't want it to get
			// auto-pruned.
			if err := os.MkdirAll(root, 0o700); err != nil {
				fmt.Fprintln(os.Stderr, "the path in $XDG_RUNTIME_DIR must be writable by the user")
				fatal(err)
			}
			if err := os.Chmod(root, os.FileMode(0o700)|os.ModeSticky); err != nil {
				fmt.Fprintln(os.Stderr, "you should check permission of the path in $XDG_RUNTIME_DIR")
				fatal(err)
			}
		}
		if err := reviseRootDir(context); err != nil {
			return err
		}
		// TODO: remove this in runc 1.3.0.
		if context.IsSet("criu") {
			fmt.Fprintln(os.Stderr, "WARNING: --criu ignored (criu binary from $PATH is used); do not use")
		}

		return configLogrus(context)
	}

	// If the command returns an error, cli takes upon itself to print
	// the error on cli.ErrWriter and exit.
	// Use our own writer here to ensure the log gets sent to the right location.
	cli.ErrWriter = &FatalWriter{cli.ErrWriter}
	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

type FatalWriter struct {
	cliErrWriter io.Writer
}

func (f *FatalWriter) Write(p []byte) (n int, err error) {
	logrus.Error(string(p))
	if !logrusToStderr() {
		return f.cliErrWriter.Write(p)
	}
	return len(p), nil
}

func configLogrus(context *cli.Context) error {
	if context.GlobalBool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
		logrus.SetReportCaller(true)
		// Shorten function and file names reported by the logger, by
		// trimming common "github.com/opencontainers/runc" prefix.
		// This is only done for text formatter.
		_, file, _, _ := runtime.Caller(0)
		prefix := filepath.Dir(file) + "/"
		logrus.SetFormatter(&logrus.TextFormatter{
			CallerPrettyfier: func(f *runtime.Frame) (string, string) {
				function := strings.TrimPrefix(f.Function, prefix) + "()"
				fileLine := strings.TrimPrefix(f.File, prefix) + ":" + strconv.Itoa(f.Line)
				return function, fileLine
			},
		})
	}

	switch f := context.GlobalString("log-format"); f {
	case "":
		// do nothing
	case "text":
		// do nothing
	case "json":
		logrus.SetFormatter(new(logrus.JSONFormatter))
	default:
		return errors.New("invalid log-format: " + f)
	}

	if file := context.GlobalString("log"); file != "" {
		f, err := os.OpenFile(file, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0o644)
		if err != nil {
			return err
		}
		logrus.SetOutput(f)
	}

	return nil
}

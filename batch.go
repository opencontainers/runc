// +build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/coreos/go-systemd/activation"
	"github.com/opencontainers/specs/specs-go"
)

var batchCommand = cli.Command{
	Name:  "batch",
	Usage: "create and run a container with a series of commands",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "config-file, c",
			Value: "config.json",
			Usage: "path to spec config file",
		},
		cli.StringFlag{
			Name:  "runtime-file, r",
			Value: "runtime.json",
			Usage: "path to runtime config file",
		},
	},
	Action: func(context *cli.Context) {
		spec, err := loadSpec(context.String("config-file"))
		if err != nil {
			fatal(err)
		}

		id := context.Args().First()
		if id == "" {
			fatal(errEmptyID)
		}

		batchFilename := context.Args().Get(1)
		if batchFilename == "" {
			fatal(fmt.Errorf("Missing batch-file-name"))
		}

		notifySocket := os.Getenv("NOTIFY_SOCKET")
		if notifySocket != "" {
			setupSdNotify(spec, notifySocket)
		}

		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}

		status, err := batchContainer(context, id, spec, batchFilename)
		if err != nil {
			logrus.Fatalf("Container start failed: %v", err)
		}
		// exit with the container's exit status so any external supervisor is
		// notified of the exit with the correct exit status.
		os.Exit(status)
	},
}

func batchContainer(context *cli.Context, id string, spec *specs.Spec, batchFilename string) (int, error) {
	var file *os.File
	var err error

	if batchFilename == "-" {
		file = os.Stdin
	} else {
		if file, err = os.Open(batchFilename); err != nil {
			return -1, err
		}
		defer file.Close()
	}

	scanner := bufio.NewScanner(file)

	container, err := createContainer(context, id, spec)
	if err != nil {
		return -1, err
	}
	defer deleteContainer(container)

	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}

	// Loop over the list of processes that people want executed.
	// Use the config Process as the template for now
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		proc := specs.Process{
			Terminal: spec.Process.Terminal,
			User:     spec.Process.User,
			Args:     strings.Split(line, " "),
			Env:      spec.Process.Env,
			Cwd:      spec.Process.Cwd,
		}

		if batchFilename == "-" {
			proc.Terminal = false
		}

		fmt.Printf("--> %q\n", proc.Args)

		rc, err := runProcess(container, &proc, listenFDs, context.String("console"), context.String("pid-file"), false)
		if rc != 0 || err != nil {
			// For now just stop on first error
			return rc, nil
		}
	}

	// All is well
	return 0, nil
}

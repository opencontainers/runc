// +build linux

package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/coreos/go-systemd/activation"
	"github.com/opencontainers/specs/specs-go"
)

var runCommand = cli.Command{
	Name:  "run",
	Usage: "create and run a single command in a new container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: "path to the root of the bundle directory",
		},
		cli.StringFlag{
			Name:  "console",
			Value: "",
			Usage: "specify the pty slave path for use with the container",
		},
		cli.BoolFlag{
			Name:  "detach,d",
			Usage: "detach from the container's process",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
	},
	Action: func(context *cli.Context) {
		id := context.Args().First()
		if id == "" {
			fatal(errEmptyID)
		}

		bundle := context.String("bundle")
		if bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				fatal(err)
			}
		}

		spec, err := loadSpec(specConfig)
		if err != nil {
			fatal(err)
		}

		notifySocket := os.Getenv("NOTIFY_SOCKET")
		if notifySocket != "" {
			setupSdNotify(spec, notifySocket)
		}

		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}

		status, err := runContainer(context, id, spec, context.Args()[1:])
		if err != nil {
			logrus.Fatalf("Container start failed: %v", err)
		}

		// exit with the container's exit status so any external supervisor is
		// notified of the exit with the correct exit status.
		os.Exit(status)
	},
}

func runContainer(context *cli.Context, id string, spec *specs.Spec, args []string) (int, error) {
	container, err := createContainer(context, id, spec)
	if err != nil {
		return -1, err
	}

	detach := context.Bool("detach")
	if !detach {
		defer deleteContainer(container) // delete when process dies
	}

	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}

	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}

	proc := specs.Process{
		Terminal: spec.Process.Terminal,
		User:     spec.Process.User,
		Args:     args,
		Env:      spec.Process.Env,
		Cwd:      spec.Process.Cwd,
	}

	return runProcess(container, &proc, listenFDs, context.String("console"), context.String("pid-file"), detach)
}

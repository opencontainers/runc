package main

import (
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

const (
	version = "0.1"
	usage   = `Open Container Project runtime

runc is a command line client for running applications packaged according to the Open Container Format (OCF) and is
a compliant implementation of the Open Container Project specification.

runc integrates well with existing process supervisors to provide a production container runtime environment for
applications. It can be used with your existing process monitoring tools and the container will be spawned as a direct
child of the process supervisor.

After creating a spec for your root filesystem with runc, you can execute a simple container in your shell by running:

    cd /mycontainer
    runc
`
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			fatal(err)
		}
		panic("--this line should never been executed, congratulations--")
	}
}

func main() {
	app := cli.NewApp()
	app.Name = "runc"
	app.Usage = usage
	app.Version = version
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "id",
			Value: getDefaultID(),
			Usage: "specify the ID to be used for the container",
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output for logging",
		},
		cli.StringFlag{
			Name:  "root",
			Value: "/var/run/ocf",
			Usage: "root directory for storage of container state (this should be located in tmpfs)",
		},
		cli.StringFlag{
			Name:  "criu",
			Value: "criu",
			Usage: "path to the criu binary used for checkpoint and restore",
		},
	}
	app.Commands = []cli.Command{
		checkpointCommand,
		eventsCommand,
		restoreCommand,
		specCommand,
	}
	app.Before = func(context *cli.Context) error {
		if context.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}
	// default action is to execute a container
	app.Action = func(context *cli.Context) {
		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}
		spec, err := loadSpec(context.Args().First())
		if err != nil {
			fatal(err)
		}
		status, err := execContainer(context, spec)
		if err != nil {
			logrus.Fatalf("Container start failed: %v", err)
		}
		// exit with the container's exit status so any external supervisor is
		// notified of the exit with the correct exit status.
		os.Exit(status)
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

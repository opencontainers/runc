package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

const (
	version = "0.2"
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

	app.Action = runAction

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

package main

import (
	"syscall"

	"github.com/codegangsta/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "start signals a created container to execute the users defined process",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The start command signals the container to start the user's defined process.`,
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		if err := container.Signal(syscall.SIGCONT); err != nil {
			fatal(err)
		}
	},
}

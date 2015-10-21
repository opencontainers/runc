// +build linux

package main

import (
	"github.com/codegangsta/cli"
)

var listimagesCommand = cli.Command{
	Name:  "listimages",
	Usage: "list the checkpointed images of a container",
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}

		if err := container.Listimages(); err != nil {
			fatal(err)
		}
	},
}

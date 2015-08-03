// +build linux

package main

import (
	"fmt"
	"github.com/codegangsta/cli"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a container",
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(fmt.Errorf("%s", err))
		}
		sigStr := context.Args().First()
		state, err := container.Status()
		if err != nil {
			fatal(fmt.Errorf("Container not running %d", state))
			// return here
		}
		errVar := container.Kill(sigStr)
		if errVar != nil {
			fatal(fmt.Errorf("%s", errVar))
		}

	},
}

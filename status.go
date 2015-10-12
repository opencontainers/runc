// +build linux

package main

import (
	"fmt"
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var statusCommand = cli.Command{
	Name:  "status",
	Usage: "Current status of container",
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		status, err := container.Status()
		if err != nil {
			logrus.Error(err)
		}
		fmt.Printf("Container ID %s %s\n", context.GlobalString("id"), status)
	},
}

// +build !linux

package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func getDefaultID() string {
	return ""
}

var (
	checkpointCommand cli.Command
	eventsCommand     cli.Command
	restoreCommand    cli.Command
	specCommand       cli.Command
)

func runAction(*cli.Context) {
	logrus.Fatal("Current OS is not supported yet")
}

// +build !linux,!solaris

package main

import "github.com/urfave/cli"

var (
	checkpointCommand cli.Command
	eventsCommand     cli.Command
	restoreCommand    cli.Command
	specCommand       cli.Command
	killCommand       cli.Command
	deleteCommand     cli.Command
	execCommand       cli.Command
	initCommand       cli.Command
	listCommand       cli.Command
	pauseCommand      cli.Command
	psCommand         cli.Command
	resumeCommand     cli.Command
	runCommand        cli.Command
	stateCommand      cli.Command
	updateCommand     cli.Command
)

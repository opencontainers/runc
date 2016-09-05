// +build solaris

package main

import "github.com/spf13/cobra"

var (
	checkpointCommand cobra.Command
	eventsCommand     cobra.Command
	restoreCommand    cobra.Command
	specCommand       cobra.Command
	killCommand       cobra.Command
	deleteCommand     cobra.Command
	execCommand       cobra.Command
	initCommand       cobra.Command
	listCommand       cobra.Command
	pauseCommand      cobra.Command
	resumeCommand     cobra.Command
	startCommand      cobra.Command
	stateCommand      cobra.Command
)

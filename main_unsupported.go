// +build !linux,!solaris

package main

import "github.com/spf13/cobra"

var (
	checkpointCommand cobra.Command
	eventsCommand     cobra.Command
	restoreCommand    cobra.Command
	specCommand       cobra.Command
	killCommand       cobra.Command
)

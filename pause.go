package main

import (
	"context"

	"github.com/urfave/cli/v3"
)

var pauseCommand = &cli.Command{
	Name:  "pause",
	Usage: "pause suspends all processes inside the container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
paused. `,
	Description: `The pause command suspends all processes in the instance of the container.

Use runc list to identify instances of containers and their current status.`,
	// Disable comma as separator for slice flags.
	DisableSliceFlagSeparator: true,
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(cmd)
		if err != nil {
			return err
		}
		err = container.Pause()
		if err != nil {
			maybeLogCgroupWarning("pause", err)
			return err
		}
		return nil
	},
}

var resumeCommand = &cli.Command{
	Name:  "resume",
	Usage: "resumes all processes that have been previously paused",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
	Description: `The resume command resumes all processes in the instance of the container.

Use runc list to identify instances of containers and their current status.`,
	// Disable comma as separator for slice flags.
	DisableSliceFlagSeparator: true,
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(cmd)
		if err != nil {
			return err
		}
		err = container.Resume()
		if err != nil {
			maybeLogCgroupWarning("resume", err)
			return err
		}
		return nil
	},
}

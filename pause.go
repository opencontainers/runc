// +build linux

package main

import (
	"fmt"

	"github.com/urfave/cli"
)

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "pause suspends all processes inside the container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
paused. `,
	Description: `The pause command suspends all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		if err := container.Pause(); err != nil {
			return err
		}
		processes, err := container.Processes()
		if err != nil {
			return err
		}
		// It's possible that container is running when we checked the current
		// status in `container.Pause()` then container process exited, and we
		// paused container successfully before `runc delete` had deleted cgroup
		// directory.
		// And it's also wanted that after `runc pause` succeed, we should get
		// a paused container instead of an exited container, so check container
		// processes after `container.Pause()` succeed.
		if len(processes) == 0 {
			return fmt.Errorf("container process already exited.")
		}

		return nil
	},
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "resumes all processes that have been previously paused",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
	Description: `The resume command resumes all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		if err := container.Resume(); err != nil {
			return err
		}

		return nil
	},
}

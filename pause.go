// +build linux

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli"
)

var pauseCommand = cli.Command{
	Name:  "pause",
	Usage: "pause suspends all processes inside the container",
	ArgsUsage: `<container-id> [container-id...]

Where "<container-id>" is the name for the instance of the container to be
paused. `,
	Description: `The pause command suspends all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	Action: func(context *cli.Context) error {
		var failedOnes []string
		if !context.Args().Present() {
			return fmt.Errorf("runc: \"pause\" requires a minimum of 1 argument")
		}

		factory, err := loadFactory(context)
		if err != nil {
			return err
		}

		for _, id := range context.Args() {
			container, err := factory.Load(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				failedOnes = append(failedOnes, id)
				continue
			}
			if err := container.Pause(); err != nil {
				fmt.Fprintf(os.Stderr, "pause container %s : %s\n", id, err)
				failedOnes = append(failedOnes, id)
			}
		}

		if len(failedOnes) > 0 {
			return fmt.Errorf("failed to pause containers: %s", strings.Join(failedOnes, ","))
		}
		return nil
	},
}

var resumeCommand = cli.Command{
	Name:  "resume",
	Usage: "resumes all processes that have been previously paused",
	ArgsUsage: `<container-id> [container-id...]

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
	Description: `The resume command resumes all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	Action: func(context *cli.Context) error {
		var failedOnes []string
		if !context.Args().Present() {
			return fmt.Errorf("runc: \"resume\" requires a minimum of 1 argument")
		}

		factory, err := loadFactory(context)
		if err != nil {
			return err
		}

		for _, id := range context.Args() {
			container, err := factory.Load(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				failedOnes = append(failedOnes, id)
				continue
			}
			if err := container.Resume(); err != nil {
				fmt.Fprintf(os.Stderr, "resume container %s : %s\n", id, err)
				failedOnes = append(failedOnes, id)
			}
		}

		if len(failedOnes) > 0 {
			return fmt.Errorf("failed to resume containers: %s", strings.Join(failedOnes, ","))
		}
		return nil
	},
}

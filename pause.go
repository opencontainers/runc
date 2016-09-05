// +build linux

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var pauseCmd = &cobra.Command{
	Short: "pause suspends all processes inside the container",
	Use: `pause <container-id>

Where "<container-id>" is the name for the instance of the container to be
paused. `,
	Long: `The pause command suspends all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasError := false
		if len(args) < 1 {
			return fmt.Errorf("runc: \"pause\" requires a minimum of 1 argument")
		}

		factory, err := loadFactory(cmd.Flags())
		if err != nil {
			return err
		}

		for _, id := range args {
			container, err := factory.Load(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				hasError = true
				continue
			}
			if err := container.Pause(); err != nil {
				fmt.Fprintf(os.Stderr, "pause container %s : %s\n", id, err)
				hasError = true
			}
		}

		if hasError {
			return fmt.Errorf("one or more of container pause failed")
		}
		return nil
	},
}

var resumeCmd = &cobra.Command{
	Short: "resumes all processes that have been previously paused",
	Use: `resume <container-id>

Where "<container-id>" is the name for the instance of the container to be
resumed.`,
	Long: `The resume command resumes all processes in the instance of the container.

Use runc list to identiy instances of containers and their current status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasError := false
		if len(args) < 1 {
			return fmt.Errorf("runc: \"resume\" requires a minimum of 1 argument")
		}

		factory, err := loadFactory(cmd.Flags())
		if err != nil {
			return err
		}

		for _, id := range args {
			container, err := factory.Load(id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				hasError = true
				continue
			}
			if err := container.Resume(); err != nil {
				fmt.Fprintf(os.Stderr, "resume container %s : %s\n", id, err)
				hasError = true
			}
		}

		if hasError {
			return fmt.Errorf("one or more of container resume failed")
		}
		return nil
	},
}

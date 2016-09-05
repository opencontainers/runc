package main

import (
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Short: "executes the user defined process in a created container",
	Use: `start <container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Long: `The start command executes the user defined process in a created container.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasError := false
		if len(args) < 1 {
			return fmt.Errorf("runc: \"start\" requires a minimum of 1 argument")
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
			status, err := container.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status for %s: %v\n", id, err)
				hasError = true
				continue
			}
			switch status {
			case libcontainer.Created:
				if err := container.Exec(); err != nil {
					fmt.Fprintf(os.Stderr, "start for %s failed: %v\n", id, err)
					hasError = true
				}
			case libcontainer.Stopped:
				fmt.Fprintln(os.Stderr, "cannot start a container that has run and stopped")
				hasError = true
			case libcontainer.Running:
				fmt.Fprintln(os.Stderr, "cannot start an already running container")
				hasError = true
			default:
				fmt.Fprintf(os.Stderr, "cannot start a container in the %s state\n", status)
				hasError = true
			}
		}

		if hasError {
			return fmt.Errorf("one or more of container start failed")
		}
		return nil
	},
}

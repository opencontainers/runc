package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

var startCommand = cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id> [container-id...]

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The start command executes the user defined process in a created container .`,
	Action: func(context *cli.Context) error {
		var failedOnes []string
		if !context.Args().Present() {
			return fmt.Errorf("runc: \"start\" requires a minimum of 1 argument")
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
			status, err := container.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status for %s: %v\n", id, err)
				failedOnes = append(failedOnes, id)
				continue
			}
			switch status {
			case libcontainer.Created:
				if err := container.Exec(); err != nil {
					fmt.Fprintf(os.Stderr, "start for %s failed: %v\n", id, err)
					failedOnes = append(failedOnes, id)
				}
			case libcontainer.Stopped:
				fmt.Fprintln(os.Stderr, "cannot start a container that has run and stopped")
				failedOnes = append(failedOnes, id)
			case libcontainer.Running:
				fmt.Fprintln(os.Stderr, "cannot start an already running container")
				failedOnes = append(failedOnes, id)
			default:
				fmt.Fprintf(os.Stderr, "cannot start a container in the %s state\n", status)
				failedOnes = append(failedOnes, id)
			}
		}

		if len(failedOnes) > 0 {
			return fmt.Errorf("failed to start containers: %s", strings.Join(failedOnes, ","))
		}
		return nil
	},
}

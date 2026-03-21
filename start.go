package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli/v3"
)

var startCommand = &cli.Command{
	Name:  "start",
	Usage: "executes the user defined process in a created container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The start command executes the user defined process in a created container.`,
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
		status, err := container.Status()
		if err != nil {
			return err
		}
		switch status {
		case libcontainer.Created:
			notifySocket, err := notifySocketStart(cmd, os.Getenv("NOTIFY_SOCKET"), container.ID())
			if err != nil {
				return err
			}
			if err := container.Exec(); err != nil {
				return err
			}
			if notifySocket != nil {
				return notifySocket.waitForContainer(container)
			}
			return nil
		case libcontainer.Stopped:
			return errors.New("cannot start a container that has stopped")
		case libcontainer.Running:
			return errors.New("cannot start an already running container")
		default:
			return fmt.Errorf("cannot start a container in the %s state", status)
		}
	},
}

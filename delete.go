package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by the container often used with detached container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container.

EXAMPLE:
For example, if the container id is "ubuntu01" and runc list currently shows the
status of "ubuntu01" as "stopped" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runc list of containers:

       # runc delete ubuntu01`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Forcibly deletes the container if it is still running (uses SIGKILL)",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}

		id := context.Args().First()
		force := context.Bool("force")
		container, err := getContainer(context)
		if err != nil {
			if errors.Is(err, libcontainer.ErrNotExist) {
				// if there was an aborted start or something of the sort then the container's directory could exist but
				// libcontainer does not see it because the state.json file inside that directory was never created.
				path := filepath.Join(context.GlobalString("root"), id)
				if e := os.RemoveAll(path); e != nil {
					fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, e)
				}
				if force {
					return nil
				}
			}
			return err
		}
		s, err := container.Status()
		if err != nil {
			return err
		}
		switch s {
		case libcontainer.Stopped:
			// If the container is stopped, we can just destroy it.
		case libcontainer.Created:
			if err := container.EnsureKilled(); err != nil {
				return err
			}
		default:
			if !force {
				return fmt.Errorf("cannot delete container %s that is not stopped: %s", id, s)
			}
			// When --force is given, we kill all container processes and
			// then destroy the container.
			if err := container.EnsureKilled(); err != nil {
				return err
			}
		}
		return container.Destroy()
	},
}

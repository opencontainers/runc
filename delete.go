package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"

	"golang.org/x/sys/unix"
)

func killContainer(container *libcontainer.Container) error {
	_ = container.Signal(unix.SIGKILL)
	for range 100 {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(unix.Signal(0)); err != nil {
			return container.Destroy()
		}
	}
	return errors.New("container init still running")
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by the container often used with detached container",
	ArgsUsage: `<container-id> [<container-id> ...]

Where "<container-id>" is the name(s) of the container(s) to delete.

EXAMPLE:
For example, if the container ids are "ubuntu01" and "nginx01", and both are stopped, the following will delete resources for both containers:

       # runc delete ubuntu01 nginx01`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force, f",
			Usage: "Forcibly deletes the container(s) if still running (uses SIGKILL)",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}

		force := context.Bool("force")
		var errs []error

		for _, id := range context.Args() {
			container, err := getContainerByID(context, id)
			if err != nil {
				if errors.Is(err, libcontainer.ErrNotExist) {
					// if container directory exists but no state.json, remove manually
					path := filepath.Join(context.GlobalString("root"), id)
					if e := os.RemoveAll(path); e != nil {
						fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, e)
						errs = append(errs, e)
					}
					continue
				}
				errs = append(errs, err)
				continue
			}

			if force {
				if err := killContainer(container); err != nil {
					errs = append(errs, err)
				}
				continue
			}

			s, err := container.Status()
			if err != nil {
				errs = append(errs, err)
				continue
			}

			switch s {
			case libcontainer.Stopped:
				if err := container.Destroy(); err != nil {
					errs = append(errs, err)
				}
			case libcontainer.Created:
				if err := killContainer(container); err != nil {
					errs = append(errs, err)
				}
			default:
				errs = append(errs, fmt.Errorf("cannot delete container %s that is not stopped: %s", id, s))
			}
		}

		if len(errs) > 0 {
			return errors.Join(errs...)
		}
		return nil
	},
}

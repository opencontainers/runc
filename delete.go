// +build !solaris

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

func killContainer(container libcontainer.Container) error {
	container.Signal(syscall.SIGKILL, false)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(syscall.Signal(0), false); err != nil {
			destroy(container)
			return nil
		}
	}
	return fmt.Errorf("container init still running")
}

var deleteCommand = cli.Command{
	Name:  "delete",
	Usage: "delete any resources held by one or more containers often used with detached containers",
	ArgsUsage: `<container-id> [container-id...]

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
		var failedOnes []string
		if !context.Args().Present() {
			return fmt.Errorf("runc: \"delete\" requires a minimum of 1 argument")
		}

		factory, err := loadFactory(context)
		if err != nil {
			return err
		}
		for _, id := range context.Args() {
			container, err := factory.Load(id)
			if err != nil {
				if lerr, ok := err.(libcontainer.Error); ok && lerr.Code() == libcontainer.ContainerNotExists {
					// if there was an aborted start or something of the sort then the container's directory could exist but
					// libcontainer does not see it because the state.json file inside that directory was never created.
					path := filepath.Join(context.GlobalString("root"), id)
					if err := os.RemoveAll(path); err != nil {
						fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, err)
					}
					fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				}
				failedOnes = append(failedOnes, id)
				continue
			}
			s, err := container.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status for %s: %v\n", id, err)
				failedOnes = append(failedOnes, id)
				continue
			}
			switch s {
			case libcontainer.Stopped:
				destroy(container)
			case libcontainer.Created:
				err := killContainer(container)
				if err != nil {
					fmt.Fprintf(os.Stderr, "kill container %s: %v\n", id, err)
					failedOnes = append(failedOnes, id)
				}
			default:
				if context.Bool("force") {
					err := killContainer(container)
					if err != nil {
						fmt.Fprintf(os.Stderr, "kill container %s: %v\n", id, err)
						failedOnes = append(failedOnes, id)
					}
				} else {
					fmt.Fprintf(os.Stderr, "cannot delete container %s that is not stopped: %s\n", id, s)
					failedOnes = append(failedOnes, id)
				}
			}
		}

		if len(failedOnes) > 0 {
			return fmt.Errorf("failed to delete containers: %s", strings.Join(failedOnes, ","))
		}
		return nil
	},
}

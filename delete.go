// +build !solaris

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/spf13/cobra"
)

func killContainer(container libcontainer.Container) error {
	container.Signal(syscall.SIGKILL)
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := container.Signal(syscall.Signal(0)); err != nil {
			destroy(container)
			return nil
		}
	}
	return fmt.Errorf("container init still running")
}

var deleteCmd = &cobra.Command{
	Short: "delete any resources held by the container often used with detached containers",
	Use: `delete [command options] <container-id>

Where "<container-id>" is the name for the instance of the container.`,
	Example: `For example, if the container id is "ubuntu01" and runc list currently shows the
status of "ubuntu01" as "stopped" the following will delete resources held for
"ubuntu01" removing "ubuntu01" from the runc list of containers:

       # runc delete ubuntu01`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hasError := false
		if len(args) < 1 {
			return fmt.Errorf("runc: \"delete\" requires a minimum of 1 argument")
		}

		flags := cmd.Flags()

		factory, err := loadFactory(flags)
		if err != nil {
			return err
		}
		for _, id := range args {
			container, err := factory.Load(id)
			if err != nil {
				if lerr, ok := err.(libcontainer.Error); ok && lerr.Code() == libcontainer.ContainerNotExists {
					// if there was an aborted start or something of the sort then the container's directory could exist but
					// libcontainer does not see it because the state.json file inside that directory was never created.
					root, _ := flags.GetString("root")
					path := filepath.Join(root, id)
					if err := os.RemoveAll(path); err != nil {
						fmt.Fprintf(os.Stderr, "remove %s: %v\n", path, err)
					}
					fmt.Fprintf(os.Stderr, "container %s does not exist\n", id)
				}
				hasError = true
				continue
			}
			s, err := container.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status for %s: %v\n", id, err)
				hasError = true
				continue
			}
			switch s {
			case libcontainer.Stopped:
				destroy(container)
			case libcontainer.Created:
				err := killContainer(container)
				if err != nil {
					fmt.Fprintf(os.Stderr, "kill container %s: %v\n", id, err)
					hasError = true
				}
			default:
				if force, _ := flags.GetBool("force"); force {
					err := killContainer(container)
					if err != nil {
						fmt.Fprintf(os.Stderr, "kill container %s: %v\n", id, err)
						hasError = true
					}
				} else {
					fmt.Fprintf(os.Stderr, "cannot delete container %s that is not stopped: %s\n", id, s)
					hasError = true
				}
			}
		}

		if hasError {
			return fmt.Errorf("one or more of the container deletions failed")
		}
		return nil
	},
}

func init() {
	flags := deleteCmd.Flags()

	flags.BoolP("force", "f", false, "Forcibly deletes the container if it is still running (uses SIGKILL)")
}

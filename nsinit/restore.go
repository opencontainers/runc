package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/utils"
)

var restoreCommand = cli.Command{
	Name:  "restore",
	Usage: "restore a container from a previous checkpoint",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
	},
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		process, err := container.Restore()
		if err != nil {
			fatal(err)
		}
		go handleSignals(process, &tty{})
		status, err := process.Wait()
		if err != nil {
			exitError, ok := err.(*exec.ExitError)
			if ok {
				status = exitError.ProcessState
			} else {
				container.Destroy()
				fatal(err)
			}
		}
		if err := container.Destroy(); err != nil {
			fatal(err)
		}
		os.Exit(utils.ExitStatus(status.Sys().(syscall.WaitStatus)))
	},
}

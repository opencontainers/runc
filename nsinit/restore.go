package main

import (
	"os"
	"os/exec"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
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
		process := &libcontainer.Process{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		//rootuid, err := config.HostUID()
		//if err != nil {
		//fatal(err)
		//}
		rootuid := 0 // XXX
		tty, err := newTty(context, process, rootuid)
		if err != nil {
			fatal(err)
		}
		if err := tty.attach(process); err != nil {
			fatal(err)
		}
		err = container.Restore(process)
		if err != nil {
			fatal(err)
		}
		go handleSignals(process, tty)
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
		ctStatus, err := container.Status()
		if ctStatus == libcontainer.Destroyed {
			if err := container.Destroy(); err != nil {
				fatal(err)
			}
		}
		os.Exit(utils.ExitStatus(status.Sys().(syscall.WaitStatus)))
	},
}

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/utils"
)

var standardEnvironment = &cli.StringSlice{
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"HOSTNAME=nsinit",
	"TERM=xterm",
}

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "tty", Usage: "allocate a TTY to the container"},
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
		cli.StringFlag{Name: "config", Value: "container.json", Usage: "path to the configuration file"},
		cli.StringFlag{Name: "user,u", Value: "root", Usage: "set the user, uid, and/or gid for the process"},
		cli.StringSliceFlag{Name: "env", Value: standardEnvironment, Usage: "set environment variables for the process"},
	},
}

func execAction(context *cli.Context) {
	factory, err := loadFactory(context)
	if err != nil {
		fatal(err)
	}
	tty, err := newTty(context)
	if err != nil {
		fatal(err)
	}
	container, err := factory.Load(context.String("id"))
	if err != nil {
		if lerr, ok := err.(libcontainer.Error); !ok || lerr.Code() != libcontainer.ContainerNotExists {
			fatal(err)
		}
		config, err := loadConfig(context)
		if err != nil {
			fatal(err)
		}
		config.Console = tty.console.Path()
		if container, err = factory.Create(context.String("id"), config); err != nil {
			fatal(err)
		}
	}
	go handleSignals(container, tty)
	process := &libcontainer.Process{
		Args:   context.Args(),
		Env:    context.StringSlice("env"),
		User:   context.String("user"),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	tty.attach(process)
	pid, err := container.Start(process)
	if err != nil {
		fatal(err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fatal(err)
	}
	status, err := proc.Wait()
	if err != nil {
		fatal(err)
	}
	if err := container.Destroy(); err != nil {
		fatal(err)
	}
	os.Exit(utils.ExitStatus(status.Sys().(syscall.WaitStatus)))
}

func handleSignals(container libcontainer.Container, tty *tty) {
	sigc := make(chan os.Signal, 10)
	signal.Notify(sigc)
	tty.resize()
	for sig := range sigc {
		switch sig {
		case syscall.SIGWINCH:
			tty.resize()
		default:
			container.Signal(sig)
		}
	}
}

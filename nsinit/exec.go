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
	Flags: append([]cli.Flag{
		cli.BoolFlag{Name: "tty,t", Usage: "allocate a TTY to the container"},
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
		cli.StringFlag{Name: "config", Value: "container.json", Usage: "path to the configuration file"},
		cli.BoolFlag{Name: "create", Usage: "create the container's configuration on the fly with arguments"},
		cli.StringFlag{Name: "user,u", Value: "root", Usage: "set the user, uid, and/or gid for the process"},
		cli.StringFlag{Name: "cwd", Value: "", Usage: "set the current working dir"},
		cli.StringSliceFlag{Name: "env", Value: standardEnvironment, Usage: "set environment variables for the process"},
	}, createFlags...),
}

func execAction(context *cli.Context) {
	factory, err := loadFactory(context)
	if err != nil {
		fatal(err)
	}
	config, err := loadConfig(context)
	if err != nil {
		fatal(err)
	}
	rootuid, err := config.HostUID()
	if err != nil {
		fatal(err)
	}
	tty, err := newTty(context, rootuid)
	if err != nil {
		fatal(err)
	}
	created := false
	container, err := factory.Load(context.String("id"))
	if err != nil {
		if tty.console != nil {
			config.Console = tty.console.Path()
		}
		created = true
		if container, err = factory.Create(context.String("id"), config); err != nil {
			tty.Close()
			fatal(err)
		}
	}
	go handleSignals(container, tty)
	process := &libcontainer.Process{
		Args:   context.Args(),
		Env:    context.StringSlice("env"),
		User:   context.String("user"),
		Cwd:    context.String("cwd"),
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	if err := tty.attach(process); err != nil {
		fatal(err)
	}
	pid, err := container.Start(process)
	if err != nil {
		tty.Close()
		if created {
			container.Destroy()
		}
		fatal(err)
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		tty.Close()
		if created {
			container.Destroy()
		}
		fatal(err)
	}
	status, err := proc.Wait()
	if err != nil {
		tty.Close()
		if created {
			container.Destroy()
		}
		fatal(err)
	}
	if created {
		if err := container.Destroy(); err != nil {
			tty.Close()
			fatal(err)
		}
	}
	tty.Close()
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

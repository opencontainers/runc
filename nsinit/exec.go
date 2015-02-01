package main

import (
	"io"
	"os"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
	consolepkg "github.com/docker/libcontainer/console"
)

var execCommand = cli.Command{
	Name:   "exec",
	Usage:  "execute a new command inside a container",
	Action: execAction,
	Flags: []cli.Flag{
		cli.BoolFlag{Name: "tty", Usage: "allocate a TTY to the container"},
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
		cli.StringFlag{Name: "config", Value: "container.json", Usage: "path to the configuration file"},
	},
}

func execAction(context *cli.Context) {
	var (
		master  *os.File
		console string
		err     error

		sigc = make(chan os.Signal, 10)

		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr

		exitCode int
	)

	factory, err := loadFactory(context)
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
		if context.Bool("tty") {
			stdin = nil
			stdout = nil
			stderr = nil
			if master, console, err = consolepkg.CreateMasterAndConsole(); err != nil {
				fatal(err)
			}
			go io.Copy(master, os.Stdin)
			go io.Copy(os.Stdout, master)
			state, err := term.SetRawTerminal(os.Stdin.Fd())
			if err != nil {
				fatal(err)
			}
			defer term.RestoreTerminal(os.Stdin.Fd(), state)
			config.Console = console
		}
		if container, err = factory.Create(context.String("id"), config); err != nil {
			fatal(err)
		}
	}
	process := &libcontainer.Process{
		Args:   context.Args(),
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	}
	if _, err := container.Start(process); err != nil {
		fatal(err)
	}
	go func() {
		resizeTty(master)
		for sig := range sigc {
			switch sig {
			case syscall.SIGWINCH:
				resizeTty(master)
			default:
				container.Signal(sig)
			}
		}
	}()
	status, err := container.Wait()
	if err != nil {
		fatal(err)
	}
	if err := container.Destroy(); err != nil {
		fatal(err)
	}
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		fatalf("Unexpected status")
	}
	os.Exit(exitCode)
}

func resizeTty(master *os.File) {
	if master == nil {
		return
	}
	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}
	term.SetWinsize(master.Fd(), ws)
}

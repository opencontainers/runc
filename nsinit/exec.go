package main

import (
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
	consolepkg "github.com/docker/libcontainer/console"
)

type tty struct {
	master  *os.File
	console string
	state   *term.State
}

func (t *tty) Close() error {
	if t.master != nil {
		t.master.Close()
	}
	if t.state != nil {
		term.RestoreTerminal(os.Stdin.Fd(), t.state)
	}
	return nil
}

func (t *tty) set(config *configs.Config) {
	config.Console = t.console
}

func (t *tty) attach(process *libcontainer.Process) {
	if t.master != nil {
		process.Stderr = nil
		process.Stdout = nil
		process.Stdin = nil
	}
}

func (t *tty) resize() error {
	if t.master == nil {
		return nil
	}
	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return err
	}
	return term.SetWinsize(t.master.Fd(), ws)
}

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
		tty.set(config)
		if container, err = factory.Create(context.String("id"), config); err != nil {
			fatal(err)
		}
	}
	go handleSignals(container, tty)
	process := &libcontainer.Process{
		Args:   context.Args(),
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
	exit(status.Sys().(syscall.WaitStatus))
}

func exit(status syscall.WaitStatus) {
	var exitCode int
	if status.Exited() {
		exitCode = status.ExitStatus()
	} else if status.Signaled() {
		exitCode = -int(status.Signal())
	} else {
		fatalf("Unexpected status")
	}
	os.Exit(exitCode)
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

func newTty(context *cli.Context) (*tty, error) {
	if context.Bool("tty") {
		master, console, err := consolepkg.CreateMasterAndConsole()
		if err != nil {
			return nil, err
		}
		go io.Copy(master, os.Stdin)
		go io.Copy(os.Stdout, master)
		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return nil, err
		}
		return &tty{
			master:  master,
			console: console,
			state:   state,
		}, nil
	}
	return &tty{}, nil
}

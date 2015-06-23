package main

import (
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainer/runc"
	"github.com/opencontainers/runc/libcontainer"
)

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p *runc.Process) *libcontainer.Process {
	return &libcontainer.Process{
		Args:   p.Args,
		Env:    p.Env,
		User:   p.User,
		Cwd:    p.Cwd,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
}

func start(context *cli.Context, spec *runc.LinuxSpec) (int, error) {
	if len(spec.Processes) != 1 {
		return -1, fmt.Errorf("runc only supports one(1) process for the container")
	}
	config, err := spec.NewConfig()
	if err != nil {
		return -1, err
	}
	rootuid, err := config.HostUID()
	if err != nil {
		return -1, err
	}
	factory, err := runc.NewFactory(context.GlobalString("root"), context.GlobalString("criu"))
	if err != nil {
		return -1, err
	}
	container, err := factory.Create(context.GlobalString("id"), config)
	if err != nil {
		return -1, err
	}
	// ensure that the container is always removed if we were the process
	// that created it.
	defer destroy(container)
	process := newProcess(spec.Processes[0])
	tty, err := runc.NewTTY(spec.Processes[0].TTY, process, rootuid)
	if err != nil {
		return -1, err
	}
	handler := runc.NewSignalHandler(tty)
	defer handler.Close()
	if err := container.Start(process); err != nil {
		return -1, err
	}
	return handler.Forward(process)
}

func destroy(container libcontainer.Container) {
	status, err := container.Status()
	if err != nil {
		logrus.Error(err)
	}
	if status != libcontainer.Checkpointed {
		if err := container.Destroy(); err != nil {
			logrus.Error(err)
		}
	}
}

// fatal prints the error's details if it is a libcontainer specific error type
// then exists the program with an exit status of 1.
func fatal(err error) {
	if lerr, ok := err.(libcontainer.Error); ok {
		lerr.Detail(os.Stderr)
		os.Exit(1)
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// fatalf formats the errror string with the specified template then exits the
// program with an exit status of 1.
func fatalf(t string, v ...interface{}) {
	fmt.Fprintf(os.Stderr, t, v...)
	os.Exit(1)
}

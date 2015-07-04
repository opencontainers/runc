// +build linux

package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/specs"
)

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			fatal(err)
		}
		panic("--this line should never been executed, congratulations--")
	}
}

func execContainer(context *cli.Context, spec *specs.LinuxSpec) (int, error) {
	config, err := createLibcontainerConfig(spec)
	if err != nil {
		return -1, err
	}
	if _, err := os.Stat(config.Rootfs); err != nil {
		if os.IsNotExist(err) {
			return -1, fmt.Errorf("Rootfs (%q) does not exist", config.Rootfs)
		}
		return -1, err
	}
	rootuid, err := config.HostUID()
	if err != nil {
		return -1, err
	}
	factory, err := loadFactory(context)
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
	process := newProcess(spec.Process)
	tty, err := newTty(spec.Process.Terminal, process, rootuid)
	if err != nil {
		return -1, err
	}
	handler := newSignalHandler(tty)
	defer handler.Close()
	if err := container.Start(process); err != nil {
		return -1, err
	}
	return handler.forward(process)
}

// default action is to execute a container
func runAction(context *cli.Context) {
	spec, err := loadSpec(context.Args().First())
	if err != nil {
		fatal(err)
	}
	if os.Geteuid() != 0 {
		logrus.Fatal("runc should be run as root")
	}
	status, err := execContainer(context, spec)
	if err != nil {
		logrus.Fatalf("Container start failed: %v", err)
	}
	// exit with the container's exit status so any external supervisor is
	// notified of the exit with the correct exit status.
	os.Exit(status)
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

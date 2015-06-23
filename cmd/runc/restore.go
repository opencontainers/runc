package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainer/runc"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
)

var restoreCommand = cli.Command{
	Name:  "restore",
	Usage: "restore a container from a previous checkpoint",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "image-path", Value: "", Usage: "path to criu image files for restoring"},
		cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
		cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
		cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
		cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
	},
	Action: func(context *cli.Context) {
		imagePath := context.String("image-path")
		if imagePath == "" {
			imagePath = runc.DefaultImagePath()
		}
		spec, err := runc.NewSpec(context.Args().First())
		if err != nil {
			fatal(err)
		}
		config, err := spec.NewConfig()
		if err != nil {
			fatal(err)
		}
		status, err := restoreContainer(context, spec, config, imagePath)
		if err != nil {
			fatal(err)
		}
		os.Exit(status)
	},
}

func restoreContainer(context *cli.Context, spec *runc.LinuxSpec, config *configs.Config, imagePath string) (code int, err error) {
	rootuid := 0
	factory, err := runc.NewFactory(context.GlobalString("root"), context.GlobalString("criu"))
	if err != nil {
		return -1, err
	}
	id := context.GlobalString("id")
	container, err := factory.Load(id)
	if err != nil {
		container, err = factory.Create(id, config)
		if err != nil {
			return -1, err
		}
	}
	options := criuOptions(context)
	// ensure that the container is always removed if we were the process
	// that created it.
	defer func() {
		if err != nil {
			return
		}
		status, err := container.Status()
		if err != nil {
			logrus.Error(err)
		}
		if status != libcontainer.Checkpointed {
			if err := container.Destroy(); err != nil {
				logrus.Error(err)
			}
			if err := os.RemoveAll(options.ImagesDirectory); err != nil {
				logrus.Error(err)
			}
		}
	}()
	process := &libcontainer.Process{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	tty, err := runc.NewTTY(spec.Processes[0].TTY, process, rootuid)
	if err != nil {
		return -1, err
	}
	defer tty.Close()
	go handleSignals(process, tty)
	if err := container.Restore(process, options); err != nil {
		return -1, err
	}
	status, err := process.Wait()
	if err != nil {
		return -1, err
	}
	return utils.ExitStatus(status.Sys().(syscall.WaitStatus)), nil
}

func criuOptions(context *cli.Context) *libcontainer.CriuOpts {
	imagePath := getCheckpointImagePath(context)
	if err := os.MkdirAll(imagePath, 0655); err != nil {
		fatal(err)
	}
	return &libcontainer.CriuOpts{
		ImagesDirectory:         imagePath,
		WorkDirectory:           context.String("work-path"),
		LeaveRunning:            context.Bool("leave-running"),
		TcpEstablished:          true, // context.Bool("tcp-established"),
		ExternalUnixConnections: context.Bool("ext-unix-sk"),
		ShellJob:                context.Bool("shell-job"),
	}
}

// we have to use this type of signal handler because there is a memory leak if we
// wait and reap with SICHLD.
func handleSignals(process *libcontainer.Process, tty *runc.TTY) {
	sigc := make(chan os.Signal, 10)
	signal.Notify(sigc)
	tty.Resize()
	for sig := range sigc {
		switch sig {
		case syscall.SIGWINCH:
			tty.Resize()
		default:
			process.Signal(sig)
		}
	}
}

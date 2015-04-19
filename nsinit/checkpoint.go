package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
)

var checkpointCommand = cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a running container",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
		cli.StringFlag{Name: "image-path", Value: "", Usage: "path for saving criu image files"},
		cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
		cli.BoolFlag{Name: "leave-running", Usage: "leave the process running after checkpointing"},
		cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
		cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
		cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
	},
	Action: func(context *cli.Context) {
		imagePath := context.String("image-path")
		if imagePath == "" {
			fatal(fmt.Errorf("The --image-path option isn't specified"))
		}

		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}
		if err := container.Checkpoint(&libcontainer.CriuOpts{
			ImagesDirectory:         imagePath,
			WorkDirectory:           context.String("work-path"),
			LeaveRunning:            context.Bool("leave-running"),
			TcpEstablished:          context.Bool("tcp-established"),
			ExternalUnixConnections: context.Bool("ext-unix-sk"),
			ShellJob:                context.Bool("shell-job"),
		}); err != nil {
			fatal(err)
		}
	},
}

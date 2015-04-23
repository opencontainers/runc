package main

import (
	"fmt"
	"strconv"
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
		cli.StringFlag{Name: "page-server", Value: "", Usage: "IP address of the page server"},
		cli.StringFlag{Name: "port", Value: "", Usage: "port number of the page server"},
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

		// these are the mandatory criu options for a container
		criuOpts := &libcontainer.CriuOpts{
			ImagesDirectory:         imagePath,
			WorkDirectory:           context.String("work-path"),
			LeaveRunning:            context.Bool("leave-running"),
			TcpEstablished:          context.Bool("tcp-established"),
			ExternalUnixConnections: context.Bool("ext-unix-sk"),
			ShellJob:                context.Bool("shell-job"),
		}

		// xxx following criu opts are optional
		// The dump image can be sent to a criu page server
		var port string
		if psAddress := context.String("page-server"); psAddress != "" {
			if port = context.String("port"); port == "" {
				fatal(fmt.Errorf("The --port number isn't specified"))
			}

			port_int, err := strconv.Atoi(port)
			if err != nil {
				fatal(fmt.Errorf("Invalid port number"))
			}
			criuOpts.Ps = &libcontainer.CriuPageServerInfo{
				Address: psAddress,
				Port:    int32(port_int),
			}
		}

		if err := container.Checkpoint(criuOpts); err != nil {
			fatal(err)
		}
	},
}

// +build linux

package main

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "execute new process inside the container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "console",
			Usage: "specify the pty slave path for use with the container",
		},
		cli.StringFlag{
			Name:  "cwd",
			Usage: "current working directory in the container",
		},
		cli.StringSliceFlag{
			Name:  "env, e",
			Usage: "set environment variables",
		},
		cli.BoolFlag{
			Name:  "tty, t",
			Usage: "allocate a pseudo-TTY",
		},
		cli.StringFlag{
			Name:  "user, u",
			Usage: "UID (format: <uid>[:<gid>])",
		},
	},
	Action: func(context *cli.Context) {
		if len(context.Args()) == 0 {
			logrus.Fatal("pass the command")
		}
		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}
		status, err := execProcess(context, context.Args())
		if err != nil {
			logrus.Fatalf("exec failed: %v", err)
		}
		os.Exit(status)
	},
}

func execProcess(context *cli.Context, args []string) (int, error) {
	container, err := getContainer(context)
	if err != nil {
		return -1, err
	}

	bundle := container.Config().Rootfs
	if err := os.Chdir(path.Dir(bundle)); err != nil {
		return -1, err
	}
	spec, _, err := loadSpec(specConfig, runtimeConfig)
	if err != nil {
		return -1, err
	}

	p := spec.Process

	// override the cwd, if passed
	if context.String("cwd") != "" {
		p.Cwd = context.String("cwd")
	}

	// append the passed env variables
	for _, e := range context.StringSlice("env") {
		p.Env = append(p.Env, e)
	}

	// set the tty
	if context.IsSet("tty") {
		p.Terminal = context.Bool("tty")
	}

	// override the user, if passed
	if context.String("user") != "" {
		u := strings.SplitN(context.String("user"), ":", 2)
		if len(u) > 1 {
			gid, err := strconv.Atoi(u[1])
			if err != nil {
				return -1, fmt.Errorf("parsing %s as int for gid failed: %v", u[1], err)
			}
			p.User.GID = uint32(gid)
		}
		uid, err := strconv.Atoi(u[0])
		if err != nil {
			return -1, fmt.Errorf("parsing %s as int for uid failed: %v", u[0], err)
		}
		p.User.UID = uint32(uid)
	}

	process := newProcess(p)
	process.Args = args
	rootuid, err := container.Config().HostUID()
	if err != nil {
		return -1, err
	}

	tty, err := newTty(p.Terminal, process, rootuid, context.String("console"))
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

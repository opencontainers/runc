package main

import (
	"github.com/docker/libcontainer/system"
	"log"
	"os"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
)

var (
	initCommand = cli.Command{
		Name:   "init",
		Usage:  "runs the init process inside the namespace",
		Action: initAction,
		Flags: []cli.Flag{
			cli.IntFlag{"fd", 0, "internal pipe fd"},
		},
	}
)

func initAction(context *cli.Context) {
	factory, err := libcontainer.New("", []string{})
	if err != nil {
		log.Fatal(err)
	}

	if context.Int("fd") == 0 {
		log.Fatal("--fd must be specified for init process")
	}

	fd := uintptr(context.Int("fd"))

	if err := factory.StartInitialization(fd); err != nil {
		log.Fatal(err)
	}

	args := []string(context.Args())

	if err := system.Execv(args[0], args[0:], os.Environ()); err != nil {
		log.Fatal(err)
	}
}

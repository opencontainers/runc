package main

import (
	"log"
	"runtime"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	_ "github.com/docker/libcontainer/nsenter"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "runs the init process inside the namespace",
	Flags: []cli.Flag{
		cli.IntFlag{Name: "fd", Value: 0, Usage: "internal pipe fd"},
	},
	Action: func(context *cli.Context) {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, err := libcontainer.New("", nil)
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
		panic("This line should never been executed")
	},
}

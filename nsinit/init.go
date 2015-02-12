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
	Action: func(context *cli.Context) {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, err := libcontainer.New("", nil)
		if err != nil {
			log.Fatal(err)
		}
		if err := factory.StartInitialization(3); err != nil {
			log.Fatal(err)
		}
		panic("This line should never been executed")
	},
}

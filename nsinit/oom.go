package main

import (
	"log"

	"github.com/codegangsta/cli"
)

var oomCommand = cli.Command{
	Name:   "oom",
	Usage:  "display oom notifications for a container",
	Action: oomAction,
}

func oomAction(context *cli.Context) {
	factory, err := loadFactory(context)
	if err != nil {
		log.Fatal(err)
	}
	container, err := factory.Load("nsinit")
	if err != nil {
		log.Fatal(err)
	}
	n, err := container.OOM()
	if err != nil {
		log.Fatal(err)
	}
	for range n {
		log.Printf("OOM notification received")
	}
}

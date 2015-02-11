package main

import (
	"log"

	"github.com/codegangsta/cli"
)

var oomCommand = cli.Command{
	Name:  "oom",
	Usage: "display oom notifications for a container",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "id", Value: "nsinit", Usage: "specify the ID for a container"},
	},
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			log.Fatal(err)
		}
		n, err := container.NotifyOOM()
		if err != nil {
			log.Fatal(err)
		}
		for range n {
			log.Printf("OOM notification received")
		}
	},
}

package main

import (
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
)

var oomCommand = cli.Command{
	Name:   "oom",
	Usage:  "display oom notifications for a container",
	Action: oomAction,
}

func oomAction(context *cli.Context) {
	state, err := configs.GetState(dataPath)
	if err != nil {
		log.Fatal(err)
	}
	n, err := libcontainer.NotifyOnOOM(state)
	if err != nil {
		log.Fatal(err)
	}
	for range n {
		log.Printf("OOM notification received")
	}
}

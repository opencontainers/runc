package main

import (
	"log"

	"github.com/codegangsta/cli"
)

var pauseCommand = cli.Command{
	Name:   "pause",
	Usage:  "pause the container's processes",
	Action: pauseAction,
}

var unpauseCommand = cli.Command{
	Name:   "unpause",
	Usage:  "unpause the container's processes",
	Action: unpauseAction,
}

func pauseAction(context *cli.Context) {
	container, err := getContainer(context)
	if err != nil {
		log.Fatal(err)
	}

	if err = container.Pause(); err != nil {
		log.Fatal(err)
	}
}

func unpauseAction(context *cli.Context) {
	container, err := getContainer(context)
	if err != nil {
		log.Fatal(err)
	}

	if err = container.Resume(); err != nil {
		log.Fatal(err)
	}
}

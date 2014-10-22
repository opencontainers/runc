package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
)

var statsCommand = cli.Command{
	Name:   "stats",
	Usage:  "display statistics for the container",
	Action: statsAction,
}

func statsAction(context *cli.Context) {
	factory, err := libcontainer.New(context.GlobalString("root"))
	if err != nil {
		log.Fatal(err)
	}

	container, err := factory.Load(context.Args().First())
	if err != nil {
		log.Fatal(err)
	}

	stats, err := container.Stats()
	if err != nil {
		log.Fatal(err)
	}
	data, jerr := json.MarshalIndent(stats, "", "\t")
	if err != nil {
		log.Fatal(jerr)
	}

	fmt.Printf("%s", data)
}

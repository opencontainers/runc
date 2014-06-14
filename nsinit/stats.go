package main

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/api"
	"github.com/docker/libcontainer/network"
)

var statsCommand = cli.Command{
	Name:   "stats",
	Usage:  "display statistics for the container",
	Action: statsAction,
}

func statsAction(context *cli.Context) {
	container, err := loadContainer()
	if err != nil {
		log.Fatal(err)
	}

	networkRuntimeInfo, err := loadNetworkRuntimeInfo()
	if err != nil {
		log.Fatal(err)
	}

	stats, err := getContainerStats(container, &networkRuntimeInfo)
	if err != nil {
		log.Fatalf("Failed to get stats - %v\n", err)
	}

	fmt.Printf("Stats:\n%v\n", stats)
}

// returns the container stats in json format.
func getContainerStats(container *libcontainer.Config, networkRuntimeInfo *network.NetworkRuntimeInfo) (string, error) {
	stats, err := libcontainer.GetContainerStats(container, networkRuntimeInfo)
	if err != nil {
		return "", err
	}

	out, err := json.MarshalIndent(stats, "", "\t")
	if err != nil {
		return "", err
	}

	return string(out), nil
}

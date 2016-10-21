// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/urfave/cli"
)

var stateCommand = cli.Command{
	Name:  "state",
	Usage: "output the state of a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container.`,
	Description: `The state command outputs current state information for the
instance of a container.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output, o",
			Usage: "select one of: ociVersion, pid, status, bundle, rootfs, created",
			Value: "",
		},
	},
	Action: func(context *cli.Context) error {
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		containerStatus, err := container.Status()
		if err != nil {
			return err
		}
		state, err := container.State()
		if err != nil {
			return err
		}
		pid := state.BaseState.InitProcessPid
		if containerStatus == libcontainer.Stopped {
			pid = 0
		}
		bundle, annotations := utils.Annotations(state.Config.Labels)
		cs := containerState{
			Version:        state.BaseState.Config.Version,
			ID:             state.BaseState.ID,
			InitProcessPid: pid,
			Status:         containerStatus.String(),
			Bundle:         bundle,
			Rootfs:         state.BaseState.Config.Rootfs,
			Created:        state.BaseState.Created,
			Annotations:    annotations,
		}

		switch context.String("output") {
		case "":
			data, err := json.MarshalIndent(cs, "", "  ")
			if err != nil {
				return err
			}
			os.Stdout.Write(data)
		case "ociVersion":
			fmt.Println(cs.Version)
		case "pid":
			fmt.Println(cs.InitProcessPid)
		case "status":
			fmt.Println(cs.Status)
		case "bundle":
			fmt.Println(cs.Bundle)
		case "rootfs":
			fmt.Println(cs.Rootfs)
		case "created":
			fmt.Println(cs.Created)
		default:
			return fmt.Errorf("invalid output option")
		}
		return nil
	},
}

// +build linux

package main

import (
	"encoding/json"
	"os"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/spf13/cobra"
)

var stateCmd = &cobra.Command{
	Short: "output the state of a container",
	Use: `state [command options] <container-id>

Where "<container-id>" is your name for the instance of the container.`,
	Long: `The state command outputs current state information for the
instance of a container.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		container, err := getContainer(cmd.Flags(), args)
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
		data, err := json.MarshalIndent(cs, "", "  ")
		if err != nil {
			return err
		}
		os.Stdout.Write(data)
		return nil
	},
}

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/moby/sys/user"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/urfave/cli"
)

const formatOptions = `table or json`

// containerState represents the platform agnostic pieces relating to a
// running container's status and state
type containerState struct {
	// Version is the OCI version for the container
	Version string `json:"ociVersion"`
	// ID is the container ID
	ID string `json:"id"`
	// InitProcessPid is the init process id in the parent namespace
	InitProcessPid int `json:"pid"`
	// Status is the current status of the container, running, paused, ...
	Status string `json:"status"`
	// Bundle is the path on the filesystem to the bundle
	Bundle string `json:"bundle"`
	// Rootfs is a path to a directory containing the container's root filesystem.
	Rootfs string `json:"rootfs"`
	// Created is the unix timestamp for the creation time of the container in UTC
	Created time.Time `json:"created"`
	// Annotations is the user defined annotations added to the config.
	Annotations map[string]string `json:"annotations,omitempty"`
	// The owner of the state directory (the owner of the container).
	Owner string `json:"owner"`
}

var listCommand = cli.Command{
	Name:  "list",
	Usage: "lists containers started by runc with the given root",
	ArgsUsage: `

Where the given root is specified via the global option "--root"
(default: "/run/runc").

EXAMPLE 1:
To list containers created via the default "--root":
       # runc list

EXAMPLE 2:
To list containers created using a non-default value for "--root":
       # runc --root value list`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "table",
			Usage: `select one of: ` + formatOptions,
		},
		cli.BoolFlag{
			Name:  "quiet, q",
			Usage: "display only container IDs",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 0, exactArgs); err != nil {
			return err
		}
		s, err := getContainers(context)
		if err != nil {
			return err
		}

		if context.Bool("quiet") {
			for _, item := range s {
				fmt.Println(item.ID)
			}
			return nil
		}

		switch context.String("format") {
		case "table":
			w := tabwriter.NewWriter(os.Stdout, 12, 1, 3, ' ', 0)
			fmt.Fprint(w, "ID\tPID\tSTATUS\tBUNDLE\tCREATED\tOWNER\n")
			for _, item := range s {
				fmt.Fprintf(w, "%s\t%d\t%s\t%s\t%s\t%s\n",
					item.ID,
					item.InitProcessPid,
					item.Status,
					item.Bundle,
					item.Created.Format(time.RFC3339Nano),
					item.Owner)
			}
			if err := w.Flush(); err != nil {
				return err
			}
		case "json":
			if err := json.NewEncoder(os.Stdout).Encode(s); err != nil {
				return err
			}
		default:
			return errors.New("invalid format option")
		}
		return nil
	},
}

func getContainers(context *cli.Context) ([]containerState, error) {
	root := context.GlobalString("root")
	list, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && context.IsSet("root") {
			// Ignore non-existing default root directory
			// (no containers created yet).
			return nil, nil
		}
		// Report other errors, including non-existent custom --root.
		return nil, err
	}
	var s []containerState
	for _, item := range list {
		if !item.IsDir() {
			continue
		}
		st, err := item.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Possible race with runc delete.
				continue
			}
			return nil, err
		}
		// This cast is safe on Linux.
		uid := st.Sys().(*syscall.Stat_t).Uid
		owner, err := user.LookupUid(int(uid))
		if err != nil {
			owner.Name = fmt.Sprintf("#%d", uid)
		}

		container, err := libcontainer.Load(root, item.Name())
		if err != nil {
			fmt.Fprintf(os.Stderr, "load container %s: %v\n", item.Name(), err)
			continue
		}
		containerStatus, err := container.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "status for %s: %v\n", item.Name(), err)
			continue
		}
		state, err := container.State()
		if err != nil {
			fmt.Fprintf(os.Stderr, "state for %s: %v\n", item.Name(), err)
			continue
		}
		pid := state.BaseState.InitProcessPid
		if containerStatus == libcontainer.Stopped {
			pid = 0
		}
		bundle, annotations := utils.Annotations(state.Config.Labels)
		s = append(s, containerState{
			Version:        state.BaseState.Config.Version,
			ID:             state.BaseState.ID,
			InitProcessPid: pid,
			Status:         containerStatus.String(),
			Bundle:         bundle,
			Rootfs:         state.BaseState.Config.Rootfs,
			Created:        state.BaseState.Created,
			Annotations:    annotations,
			Owner:          owner.Name,
		})
	}
	return s, nil
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"sync"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/urfave/cli/v3"
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

type listResult struct {
	state   *containerState
	warning string
	err     error
}

type ownerEntry struct {
	once sync.Once
	name string
}

var listCommand = &cli.Command{
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
	// Disable comma as separator for slice flags.
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "table",
			Usage:   `select one of: ` + formatOptions,
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "display only container IDs",
		},
	},
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 0, exactArgs); err != nil {
			return err
		}
		s, err := getContainers(cmd)
		if err != nil {
			return err
		}

		if cmd.Bool("quiet") {
			for _, item := range s {
				fmt.Println(item.ID)
			}
			return nil
		}

		switch cmd.String("format") {
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

func getContainers(cmd *cli.Command) ([]containerState, error) {
	root := cmd.String("root")
	list, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !cmd.IsSet("root") {
			// Ignore non-existing default root directory
			// (no containers created yet).
			return nil, nil
		}
		// Report other errors, including non-existent custom --root.
		return nil, err
	}
	lookupOwner := newOwnerLookup(user.LookupId)
	return collectContainerStates(list, func(item os.DirEntry) listResult {
		return getContainerState(root, item, lookupOwner)
	}, os.Stderr)
}

func newOwnerLookup(lookupUser func(string) (*user.User, error)) func(uint32) string {
	var ownerMu sync.Mutex
	owners := make(map[uint32]*ownerEntry)
	return func(uid uint32) string {
		ownerMu.Lock()
		owner := owners[uid]
		if owner == nil {
			owner = &ownerEntry{}
			owners[uid] = owner
		}
		ownerMu.Unlock()
		owner.once.Do(func() {
			owner.name = "#" + strconv.Itoa(int(uid))
			if u, err := lookupUser(owner.name[1:]); err == nil {
				owner.name = u.Username
			}
		})
		return owner.name
	}
}

func collectContainerStates(list []os.DirEntry, getState func(os.DirEntry) listResult, warnings io.Writer) ([]containerState, error) {
	const maxWorkers = 4
	// Keep results in directory order so output and errors stay stable.
	results := make([]listResult, len(list))
	jobs := make(chan int)
	var workers sync.WaitGroup
	for range min(len(list), maxWorkers) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for i := range jobs {
				results[i] = getState(list[i])
			}
		}()
	}
	for i := range list {
		jobs <- i
	}
	close(jobs)
	workers.Wait()

	var s []containerState
	for _, result := range results {
		if result.err != nil {
			return nil, result.err
		}
		if result.warning != "" {
			fmt.Fprint(warnings, result.warning)
		}
		if result.state != nil {
			s = append(s, *result.state)
		}
	}
	return s, nil
}

func getContainerState(root string, item os.DirEntry, lookupOwner func(uint32) string) listResult {
	if !item.IsDir() {
		return listResult{}
	}
	st, err := item.Info()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Possible race with runc delete.
			return listResult{}
		}
		return listResult{err: err}
	}
	// This cast is safe on Linux.
	uid := st.Sys().(*syscall.Stat_t).Uid
	owner := lookupOwner(uid)

	container, err := libcontainer.Load(root, item.Name())
	if err != nil {
		return listResult{warning: fmt.Sprintf("load container %s: %v\n", item.Name(), err)}
	}
	containerStatus, err := container.Status()
	if err != nil {
		return listResult{warning: fmt.Sprintf("status for %s: %v\n", item.Name(), err)}
	}
	state, err := container.State()
	if err != nil {
		return listResult{warning: fmt.Sprintf("state for %s: %v\n", item.Name(), err)}
	}
	pid := state.BaseState.InitProcessPid
	if containerStatus == libcontainer.Stopped {
		pid = 0
	}
	bundle, annotations := utils.Annotations(state.Config.Labels)
	return listResult{state: &containerState{
		Version:        state.BaseState.Config.Version,
		ID:             state.BaseState.ID,
		InitProcessPid: pid,
		Status:         containerStatus.String(),
		Bundle:         bundle,
		Rootfs:         state.BaseState.Config.Rootfs,
		Created:        state.BaseState.Created,
		Annotations:    annotations,
		Owner:          owner,
	}}
}

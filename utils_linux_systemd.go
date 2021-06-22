// +build linux,!no_systemd

package main

import (
	"os"

	"github.com/coreos/go-systemd/v22/activation"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// retrieve fitting the cgroup backend constructor function
// this exists in two versions, one w/ systemd support, another w/o that
func getCgroupBackend(context *cli.Context) (func(*libcontainer.LinuxFactory) error, error) {
	// We default to cgroupfs, and can only use systemd if the system is a
	// systemd box.
	cgroupManager := libcontainer.Cgroupfs
	rootlessCg, err := shouldUseRootlessCgroupManager(context)
	if err != nil {
		return nil, err
	}
	if rootlessCg {
		cgroupManager = libcontainer.RootlessCgroupfs
	}
	if context.GlobalBool("systemd-cgroup") {
		if !systemd.IsRunningSystemd() {
			return nil, errors.New("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
		cgroupManager = libcontainer.SystemdCgroups
		if rootlessCg {
			cgroupManager = libcontainer.RootlessSystemdCgroups
		}
	}

	return cgroupManager, nil
}

func getListenFDs() []*os.File {
	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}
	return listenFDs
}

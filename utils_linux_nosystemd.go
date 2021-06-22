// +build linux,no_systemd

package main

import (
	"os"

	"github.com/opencontainers/runc/libcontainer"
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

	return cgroupManager, nil
}

func getListenFDs() ([]*os.File) {
	listenFDs := []*os.File{}
	return listenFDs
}

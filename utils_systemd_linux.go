//go:build linux && !runc_nosd

package main

import (
	"os"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
)

func sdGetListenFDs() []*os.File {
	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}
	return listenFDs
}

func sdDetectUID() (int, error) {
	return systemd.DetectUID()
}

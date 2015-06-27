package main

import (
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
)

var allowedDevices = []*configs.Device{}

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	root := context.GlobalString("root")
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return libcontainer.New(abs, nil, func(l *libcontainer.WindowsFactory) error {
		return nil
	})
}

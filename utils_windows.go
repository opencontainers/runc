package main

import (
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

// loadFactory returns the configured factory instance for execing containers.
func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	return nil, nil
}

func startContainer(context *cli.Context, spec *specs.Spec, create bool) (int, error) {
	return 0, nil
}

func setupSdNotify(spec *specs.Spec, notifySocket string) {
}

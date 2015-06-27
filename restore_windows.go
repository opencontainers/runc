package main

import (
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer/configs"
)

var restoreCommand = cli.Command{}

func restoreContainer(context *cli.Context, spec *WindowsSpec, config *configs.Config, imagePath string) (code int, err error) {
	return 0, nil
}

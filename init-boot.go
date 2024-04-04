package main

import (
	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

var initBootCommand = cli.Command{
	Name:  "init-boot",
	Usage: `launch the container process (do not call it outside of runc)`,
	Action: func(context *cli.Context) error {
		libcontainer.Init()
		return nil
	},
}

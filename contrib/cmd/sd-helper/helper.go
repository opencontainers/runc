package main

import (
	"errors"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// version will be populated by the Makefile, read from
// VERSION file of the source code.
var version = ""

// gitCommit will be the hash that the binary was built from
// and will be populated by the Makefile.
var gitCommit = ""

const (
	usage = `Open Container Initiative contrib/cmd/sd-helper

sd-helper is a tool that uses runc/libcontainer/cgroups/systemd package
functionality to communicate to systemd in order to perform various operations.
Currently this is limited to starting and stopping systemd transient slice
units.

Example:

	sd-helper start system-pod123.slice
`
)

func main() {
	if !systemd.IsRunningSystemd() {
		logrus.Fatal("systemd is required")
	}

	app := cli.NewApp()
	app.Name = "sd-helper"
	app.Usage = usage

	// Set version to be the same as runc.
	var v []string
	if version != "" {
		v = append(v, version)
	}
	if gitCommit != "" {
		v = append(v, "commit: "+gitCommit)
	}
	app.Version = strings.Join(v, "\n")

	// Set the flags.
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Enable debug output",
		},
		cli.StringFlag{
			Name:  "parent, p",
			Usage: "parent unit name",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "start",
			Usage: "start a transient unit",
			Action: func(c *cli.Context) error {
				return unitCommand("start", c)
			},
		},
		{
			Name:  "stop",
			Usage: "stop a transient unit",
			Action: func(c *cli.Context) error {
				return unitCommand("stop", c)
			},
		},
	}
	err := app.Run(os.Args)
	if err != nil {
		logrus.Fatal(err)
	}
}

func newManager(config *configs.Cgroup) cgroups.Manager {
	if cgroups.IsCgroup2UnifiedMode() {
		return systemd.NewUnifiedManager(config, "", false)
	}
	return systemd.NewLegacyManager(config, nil)
}

func unitCommand(cmd string, c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}
	name := c.Args().First()
	if name == "" {
		return errors.New("unit name is required")
	}

	podConfig := &configs.Cgroup{
		Name:      name,
		Parent:    c.String("parent"),
		Resources: &configs.Resources{},
	}
	pm := newManager(podConfig)

	switch cmd {
	case "start":
		return pm.Apply(-1)
	case "stop":
		return pm.Destroy()
	}

	// Should not happen.
	return errors.New("invalid command")
}

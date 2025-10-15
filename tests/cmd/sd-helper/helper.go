// sd-helper is a command-line tool to provide some very minimal helpers to
// communicate with systemd.
//
// This tool is only intended to be used within runc's integration tests.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/systemd"
)

func usage() {
	fmt.Print(`Open Container Initiative tests/cmd/sd-helper

sd-helper is a tool that uses github.com/opencontainers/groups/systemd package
functionality to communicate to systemd in order to perform various operations.
Currently this is limited to starting and stopping systemd transient slice
units.

Usage:
	sd-helper [-debug] [-parent <pname>] {start|stop} <name>

Example:
	sd-helper -parent system.slice start system-pod123.slice
`)
	os.Exit(1)
}

var (
	debug  = flag.Bool("debug", false, "enable debug output")
	parent = flag.String("parent", "", "parent unit name")
)

func main() {
	if !systemd.IsRunningSystemd() {
		logrus.Fatal("systemd is required")
	}

	// Set the flags.
	flag.Parse()
	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	if flag.NArg() != 2 {
		usage()
	}

	cmd := flag.Arg(0)
	unit := flag.Arg(1)

	err := unitCommand(cmd, unit, *parent)
	if err != nil {
		logrus.Fatal(err)
	}
}

func newManager(config *cgroups.Cgroup) (cgroups.Manager, error) {
	if cgroups.IsCgroup2UnifiedMode() {
		return systemd.NewUnifiedManager(config, "")
	}
	return systemd.NewLegacyManager(config, nil)
}

func unitCommand(cmd, name, parent string) error {
	podConfig := &cgroups.Cgroup{
		Name:      name,
		Parent:    parent,
		Resources: &cgroups.Resources{},
	}
	pm, err := newManager(podConfig)
	if err != nil {
		return err
	}

	switch cmd {
	case "start":
		return pm.Apply(-1)
	case "stop":
		return pm.Destroy()
	}

	return fmt.Errorf("unknown command: %s", cmd)
}

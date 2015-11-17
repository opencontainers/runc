// +build linux

package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/units"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/system"
)

var psCommand = cli.Command{
	Name:  "ps",
	Usage: "lists containers started by runc with the given root",
	Action: func(context *cli.Context) {

		// preload the container factory
		if factory == nil {
			err := factoryPreload(context)
			if err != nil {
				logrus.Fatal(err)
				return
			}
		}

		// get the list of containers
		root := context.GlobalString("root")
		absRoot, err := filepath.Abs(root)
		if err != nil {
			logrus.Fatal(err)
			return
		}
		list, err := ioutil.ReadDir(absRoot)

		fmt.Printf("%-19s %-19s %-12s %-10s %s\n", "CONTAINER ID", "CREATED", "STATUS", "INIT PID", "ROOT FS")

		// output containers
		for _, item := range list {
			switch {
			case !item.IsDir():
				// do nothing with misc files in the containers directory
			case item.IsDir():
				outputPSInfo(item.Name())
			}
		}

	},
}

func outputPSInfo(id string) {
	container, err := factory.Load(id)
	if err != nil {
		logrus.Fatal(err)
		return
	}

	var status string
	s, err := container.Status()
	if err != nil {
		logrus.Fatal(err)
		return
	}

	switch s {
	case libcontainer.Running:
		status = "running"
	case libcontainer.Pausing:
		status = "pausing"
	case libcontainer.Paused:
		status = "paused"
	case libcontainer.Checkpointed:
		status = "checkpointed"
	case libcontainer.Destroyed:
		status = "destroyed"
	default:
		status = "unknown"
	}

	state, err := container.State()
	if err != nil {
		logrus.Fatal(err)
		return
	}

	conf := container.Config()

	// convert the InitProcessStartTime string to seconds since boot (have to divide by clock ticks)
	pidStarted, err := strconv.ParseInt(state.BaseState.InitProcessStartTime, 10, 64)
	if err != nil {
		logrus.Fatal(err)
		return
	}
	pidStarted /= int64(system.GetClockTicks())

	//grab sysinfo to identify current uptime
	sysinfo := syscall.Sysinfo_t{}
	err = syscall.Sysinfo(&sysinfo)
	if err != nil {
		logrus.Fatal(err)
		return
	}

	// running for is the difference between pidStarted and current uptime
	// Note: time.Duration is in nanoseconds
	runningFor := units.HumanDuration((time.Duration)((sysinfo.Uptime-pidStarted)*1000000000)) + " ago"

	fmt.Printf("%-19s %-19s %-12s %-10d %s\n", container.ID(), runningFor, status, state.BaseState.InitProcessPid, conf.Rootfs)

}

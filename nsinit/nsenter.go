package main

import (
	"log"
	"strconv"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/devices"
	"github.com/docker/libcontainer/mount/nodes"
	"github.com/docker/libcontainer/namespaces"
	_ "github.com/docker/libcontainer/namespaces/nsenter"
)

// nsenterExec exec's a process inside an existing container
func nsenterExec(config *libcontainer.Config, args []string) {
	if err := namespaces.FinalizeSetns(config, args); err != nil {
		log.Fatalf("failed to nsenter: %s", err)
	}
}

// nsenterMknod runs mknod inside an existing container
//
// mknod <path> <type> <major> <minor>
func nsenterMknod(config *libcontainer.Config, args []string) {
	if len(args) != 4 {
		log.Fatalf("expected mknod to have 4 arguments not %d", len(args))
	}

	t := rune(args[1][0])

	major, err := strconv.Atoi(args[2])
	if err != nil {
		log.Fatal(err)
	}

	minor, err := strconv.Atoi(args[3])
	if err != nil {
		log.Fatal(err)
	}

	n := &devices.Device{
		Path:        args[0],
		Type:        t,
		MajorNumber: int64(major),
		MinorNumber: int64(minor),
	}

	if err := nodes.CreateDeviceNode("/", n); err != nil {
		log.Fatal(err)
	}
}

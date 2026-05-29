package libcontainer_test

import (
	"log"
	"os"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/devices/config"

	// To enable device management code, import cgroups/devices package.
	// Without it, cgroup manager won't be able to set up device access rules,
	// and will fail if devices are specified in the container configuration.
	_ "github.com/opencontainers/cgroups/devices"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"

	// Required for container enter functionality.
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

func Example_container() {
	const defaultMountFlags = unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV

	// Default set of allowed devices.
	var devices []*config.Rule
	for _, device := range specconv.AllowedDevices {
		devices = append(devices, &device.Rule)
	}
	// To create a container you first have to create a configuration
	// struct describing how the container is to be created.
	config := &configs.Config{
		Rootfs: "/your/path/to/rootfs",
		Capabilities: &configs.Capabilities{
			Bounding: []string{
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Effective: []string{
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Permitted: []string{
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
		},
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWPID},
			{Type: configs.NEWUSER},
			{Type: configs.NEWNET},
			{Type: configs.NEWCGROUP},
		}),
		Cgroups: &cgroups.Cgroup{
			Name:   "test-container",
			Parent: "system",
			Resources: &cgroups.Resources{
				MemorySwappiness: nil,
				Devices:          devices,
			},
		},
		MaskPaths: []string{
			"/proc/kcore",
			"/sys/firmware",
		},
		ReadonlyPaths: []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
		Devices:  specconv.AllowedDevices,
		Hostname: "testing",
		Mounts: []*configs.Mount{
			{
				Source:      "proc",
				Destination: "/proc",
				Device:      "proc",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "tmpfs",
				Destination: "/dev",
				Device:      "tmpfs",
				Flags:       unix.MS_NOSUID | unix.MS_STRICTATIME,
				Data:        "mode=755",
			},
			{
				Source:      "devpts",
				Destination: "/dev/pts",
				Device:      "devpts",
				Flags:       unix.MS_NOSUID | unix.MS_NOEXEC,
				Data:        "newinstance,ptmxmode=0666,mode=0620,gid=5",
			},
			{
				Device:      "tmpfs",
				Source:      "shm",
				Destination: "/dev/shm",
				Data:        "mode=1777,size=65536k",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "mqueue",
				Destination: "/dev/mqueue",
				Device:      "mqueue",
				Flags:       defaultMountFlags,
			},
			{
				Source:      "sysfs",
				Destination: "/sys",
				Device:      "sysfs",
				Flags:       defaultMountFlags | unix.MS_RDONLY,
			},
		},
		UIDMappings: []configs.IDMap{
			{
				ContainerID: 0,
				HostID:      1000,
				Size:        65536,
			},
		},
		GIDMappings: []configs.IDMap{
			{
				ContainerID: 0,
				HostID:      1000,
				Size:        65536,
			},
		},
		Networks: []*configs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		},
		Rlimits: []configs.Rlimit{
			{
				Type: unix.RLIMIT_NOFILE,
				Hard: uint64(1025),
				Soft: uint64(1025),
			},
		},
	}

	// Once you have the configuration populated you can create a container
	// with a specified ID under a specified state directory:
	container, err := libcontainer.Create("/run/containers", "container-id", config)
	if err != nil {
		log.Fatal(err)
		return
	}

	// To spawn bash as the initial process inside the container and have the
	// processes pid returned in order to wait, signal, or kill the process:
	process := &libcontainer.Process{
		Args:   []string{"/bin/bash"},
		Env:    []string{"PATH=/bin"},
		UID:    0,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Init:   true,
	}

	err = container.Run(process)
	if err != nil {
		_ = container.Destroy()
		log.Fatal(err)
		return
	}

	// Wait for the process to finish.
	_, err = process.Wait()
	if err != nil {
		log.Fatal(err)
	}

	// Destroy the container.
	err = container.Destroy()
	if err != nil {
		log.Fatal(err)
	}

	// Additional ways to interact with a running container are:

	// Return all the pids for all processes running inside the container.
	processes, err := container.Processes()
	if err != nil {
		log.Fatal(err)
	}
	log.Print(processes)

	// Get detailed cpu, memory, io, and network statistics for the container and
	// it's processes.
	stats, err := container.Stats()
	if err != nil {
		log.Fatal(err)
	}
	log.Print(stats)

	// Pause all processes inside the container.
	err = container.Pause()
	if err != nil {
		log.Fatal(err)
	}

	// Resume all paused processes.
	err = container.Resume()
	if err != nil {
		log.Fatal(err)
	}

	// Send signal to container's init process.
	err = container.Signal(unix.SIGHUP)
	if err != nil {
		log.Fatal(err)
	}

	// Update container resource constraints.
	err = container.Set(*config)
	if err != nil {
		log.Fatal(err)
	}

	// Get current status of the container.
	status, err := container.Status()
	if err != nil {
		log.Fatal(err)
	}
	log.Print(status)

	// Get current container's state information.
	state, err := container.State()
	if err != nil {
		log.Fatal(err)
	}
	log.Print(state)
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer/configs"
)

var createFlags = []cli.Flag{
	cli.IntFlag{Name: "parent-death-signal", Usage: "set the signal that will be delivered to the process in case the parent dies"},
	cli.BoolFlag{Name: "read-only", Usage: "set the container's rootfs as read-only"},
	cli.StringSliceFlag{Name: "bind", Value: &cli.StringSlice{}, Usage: "add bind mounts to the container"},
	cli.StringSliceFlag{Name: "tmpfs", Value: &cli.StringSlice{}, Usage: "add tmpfs mounts to the container"},
	cli.IntFlag{Name: "cpushares", Usage: "set the cpushares for the container"},
	cli.IntFlag{Name: "memory-limit", Usage: "set the memory limit for the container"},
	cli.IntFlag{Name: "memory-swap", Usage: "set the memory swap limit for the container"},
	cli.StringFlag{Name: "cpuset-cpus", Usage: "set the cpuset cpus"},
	cli.StringFlag{Name: "cpuset-mems", Usage: "set the cpuset mems"},
	cli.StringFlag{Name: "apparmor-profile", Usage: "set the apparmor profile"},
	cli.StringFlag{Name: "process-label", Usage: "set the process label"},
	cli.StringFlag{Name: "mount-label", Usage: "set the mount label"},
}

var configCommand = cli.Command{
	Name:  "config",
	Usage: "generate a standard configuration file for a container",
	Flags: append([]cli.Flag{
		cli.StringFlag{Name: "file,f", Value: "stdout", Usage: "write the configuration to the specified file"},
	}, createFlags...),
	Action: func(context *cli.Context) {
		template := getTemplate()
		modify(template, context)
		data, err := json.MarshalIndent(template, "", "\t")
		if err != nil {
			fatal(err)
		}
		var f *os.File
		filePath := context.String("file")
		switch filePath {
		case "stdout", "":
			f = os.Stdout
		default:
			if f, err = os.Create(filePath); err != nil {
				fatal(err)
			}
			defer f.Close()
		}
		if _, err := io.Copy(f, bytes.NewBuffer(data)); err != nil {
			fatal(err)
		}
	},
}

func modify(config *configs.Config, context *cli.Context) {
	config.ParentDeathSignal = context.Int("parent-death-signal")
	config.Readonlyfs = context.Bool("read-only")
	config.Cgroups.CpusetCpus = context.String("cpuset-cpus")
	config.Cgroups.CpusetMems = context.String("cpuset-mems")
	config.Cgroups.CpuShares = int64(context.Int("cpushares"))
	config.Cgroups.Memory = int64(context.Int("memory-limit"))
	config.Cgroups.MemorySwap = int64(context.Int("memory-swap"))
	config.AppArmorProfile = context.String("apparmor-profile")
	config.ProcessLabel = context.String("process-label")
	config.MountLabel = context.String("mount-label")
}

func getTemplate() *configs.Config {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return &configs.Config{
		Rootfs:            cwd,
		ParentDeathSignal: int(syscall.SIGKILL),
		Capabilities: []string{
			"CHOWN",
			"DAC_OVERRIDE",
			"FSETID",
			"FOWNER",
			"MKNOD",
			"NET_RAW",
			"SETGID",
			"SETUID",
			"SETFCAP",
			"SETPCAP",
			"NET_BIND_SERVICE",
			"SYS_CHROOT",
			"KILL",
			"AUDIT_WRITE",
		},
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWPID},
			{Type: configs.NEWNET},
		}),
		Cgroups: &configs.Cgroup{
			Name:            filepath.Base(cwd),
			Parent:          "nsinit",
			AllowAllDevices: false,
			AllowedDevices:  configs.DefaultAllowedDevices,
		},

		Devices:  configs.DefaultAutoCreatedDevices,
		Hostname: "nsinit",
		Networks: []*configs.Network{
			{
				Type:    "loopback",
				Address: "127.0.0.1/0",
				Gateway: "localhost",
			},
		},
		Rlimits: []configs.Rlimit{
			{
				Type: syscall.RLIMIT_NOFILE,
				Hard: 1024,
				Soft: 1024,
			},
		},
	}

}

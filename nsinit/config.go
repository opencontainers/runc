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

var configCommand = cli.Command{
	Name:  "config",
	Usage: "generate a standard configuration file for a container",
	Flags: []cli.Flag{
		cli.StringFlag{Name: "file,f", Value: "stdout", Usage: "write the configuration to the specified file"},
	},
	Action: func(context *cli.Context) {
		template := getTemplate()
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
				Hard: uint64(1024),
				Soft: uint64(1024),
			},
		},
		Mounts: []*configs.Mount{
			{
				Type:        "tmpfs",
				Destination: "/tmp",
			},
		},
	}

}

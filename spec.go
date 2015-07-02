package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/specs"
)

var specCommand = cli.Command{
	Name:  "spec",
	Usage: "create a new specification file",
	Action: func(context *cli.Context) {
		spec := specs.Spec{
			Version: version,
			Platform: specs.Platform{
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
			},
			Root: specs.Root{
				Path:     "rootfs",
				Readonly: true,
			},
			Process: specs.Process{
				Terminal: true,
				User:     specs.User{},
				Args: []string{
					"sh",
				},
				Env: []string{
					"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					"TERM=xterm",
				},
			},
			Hostname: "shell",
			Mounts: []specs.Mount{
				{
					Type:        "proc",
					Source:      "proc",
					Destination: "/proc",
					Options:     "",
				},
				{
					Type:        "tmpfs",
					Source:      "tmpfs",
					Destination: "/dev",
					Options:     "nosuid,strictatime,mode=755,size=65536k",
				},
				{
					Type:        "devpts",
					Source:      "devpts",
					Destination: "/dev/pts",
					Options:     "nosuid,noexec,newinstance,ptmxmode=0666,mode=0620,gid=5",
				},
				{
					Type:        "tmpfs",
					Source:      "shm",
					Destination: "/dev/shm",
					Options:     "nosuid,noexec,nodev,mode=1777,size=65536k",
				},
				{
					Type:        "mqueue",
					Source:      "mqueue",
					Destination: "/dev/mqueue",
					Options:     "nosuid,noexec,nodev",
				},
				{
					Type:        "sysfs",
					Source:      "sysfs",
					Destination: "/sys",
					Options:     "nosuid,noexec,nodev",
				},
			},
		}
		data, err := json.MarshalIndent(&spec, "", "\t")
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("%s", data)
	},
}

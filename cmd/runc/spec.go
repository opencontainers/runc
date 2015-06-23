package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/codegangsta/cli"
	"github.com/opencontainer/runc"
)

const cpuQuotaMultiplyer = 100000

var specCommand = cli.Command{
	Name:  "spec",
	Usage: "create a new specification file",
	Action: func(context *cli.Context) {
		spec := runc.Spec{
			Version: version,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Root: runc.Root{
				Path:     "rootfs",
				Readonly: true,
			},
			Processes: []*runc.Process{
				{
					TTY:  true,
					User: "daemon",
					Args: []string{
						"sh",
					},
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"TERM=xterm",
					},
				},
			},
			Cpus:     1.1,
			Memory:   1024,
			Hostname: "shell",
			Capabilities: []string{
				"AUDIT_WRITE",
				"KILL",
				"NET_BIND_SERVICE",
			},
			Devices: []string{
				"null",
				"random",
				"full",
				"tty",
				"zero",
				"urandom",
			},
			Namespaces: []runc.Namespace{
				{Type: "process"},
				{Type: "network"},
				{Type: "mount"},
				{Type: "ipc"},
				{Type: "uts"},
			},
			Mounts: []runc.Mount{
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
			fatal(err)
		}
		fmt.Printf("%s", data)
	},
}

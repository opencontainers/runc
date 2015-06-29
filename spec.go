package main

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/codegangsta/cli"
)

const cpuQuotaMultiplyer = 100000

type Mount struct {
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Options     string `json:"options"`
}

type Process struct {
	TTY  bool     `json:"tty"`
	User string   `json:"user"`
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Cwd  string   `json:"cwd"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type Namespace struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
}

type PortableSpec struct {
	Version   string     `json:"version"`
	OS        string     `json:"os"`
	Arch      string     `json:"arch"`
	Processes []*Process `json:"processes"`
	Root      Root       `json:"root"`
	Hostname  string     `json:"hostname"`
	Mounts    []Mount    `json:"mounts"`
}

var specCommand = cli.Command{
	Name:  "spec",
	Usage: "create a new specification file",
	Action: func(context *cli.Context) {
		spec := PortableSpec{
			Version: version,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
			Root: Root{
				Path:     "rootfs",
				Readonly: true,
			},
			Processes: []*Process{
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
			Hostname: "shell",
			Mounts: []Mount{
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

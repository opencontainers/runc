package main

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"

	"github.com/Sirupsen/logrus"
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
	Terminal bool     `json:"terminal"`
	User     string   `json:"user"`
	Args     []string `json:"args"`
	Env      []string `json:"env"`
	Cwd      string   `json:"cwd"`
}

type Root struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly"`
}

type Platform struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
}

type PortableSpec struct {
	Version  string   `json:"version"`
	Platform Platform `json:"platform"`
	Process  Process  `json:"process"`
	Root     Root     `json:"root"`
	Hostname string   `json:"hostname"`
	Mounts   []Mount  `json:"mounts"`
}

var specCommand = cli.Command{
	Name:  "spec",
	Usage: "create a new specification file",
	Action: func(context *cli.Context) {
		spec := PortableSpec{
			Version: version,
			Platform: Platform{
				OS:   runtime.GOOS,
				Arch: runtime.GOARCH,
			},
			Root: Root{
				Path:     "rootfs",
				Readonly: true,
			},
			Process: Process{
				Terminal: true,
				User:     "daemon",
				Args: []string{
					"sh",
				},
				Env: []string{
					"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					"TERM=xterm",
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
			logrus.Fatal(err)
		}
		fmt.Printf("%s", data)
	},
}

func (m *Mount) UnmarshalJSON(s []byte) error {
	type readMount Mount
	var v interface{}

	if err := json.Unmarshal(s, &v); err != nil {
		return err
	}

	switch v.(type) {

	case string:
		S := strings.Trim(string(s), "\"")
		a := strings.Split(S, " ")
		if len(a) < 3 {
			return fmt.Errorf("fstab line format needs at least 3 parameters")
		}
		*m = Mount{
			Type:        a[2],
			Source:      a[0],
			Destination: a[1],
			Options:     strings.Join(a[3:], ""),
		}
		return nil

	case map[string]interface{}:
		var rm readMount
		if err := json.Unmarshal(s, &rm); err != nil {
			return err
		}
		*m = Mount(rm)
		return nil

	default:
		return fmt.Errorf("cannot unmarshal mount from other than string or object")
	}
}

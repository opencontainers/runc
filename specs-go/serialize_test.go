package specs

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/pquerna/ffjson/ffjson"
)

func init() {
	data, err := json.Marshal(spec)
	if err != nil {
		panic(err)
	}
	encoded = data
}

var encoded []byte

var rwm = "rwm"

// sample spec for testing marshaling performance
var spec = &Spec{
	Version: Version,
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
		User:     User{},
		Args: []string{
			"sh",
		},
		Env: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"TERM=xterm",
		},
		Cwd:             "/",
		NoNewPrivileges: true,
		Capabilities: []string{
			"CAP_AUDIT_WRITE",
			"CAP_KILL",
			"CAP_NET_BIND_SERVICE",
		},
		Rlimits: []Rlimit{
			{
				Type: "RLIMIT_NOFILE",
				Hard: uint64(1024),
				Soft: uint64(1024),
			},
		},
	},
	Hostname: "runc",
	Mounts: []Mount{
		{
			Destination: "/proc",
			Type:        "proc",
			Source:      "proc",
			Options:     nil,
		},
		{
			Destination: "/dev",
			Type:        "tmpfs",
			Source:      "tmpfs",
			Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
		},
		{
			Destination: "/dev/pts",
			Type:        "devpts",
			Source:      "devpts",
			Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
		},
		{
			Destination: "/dev/shm",
			Type:        "tmpfs",
			Source:      "shm",
			Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
		},
		{
			Destination: "/dev/mqueue",
			Type:        "mqueue",
			Source:      "mqueue",
			Options:     []string{"nosuid", "noexec", "nodev"},
		},
		{
			Destination: "/sys",
			Type:        "sysfs",
			Source:      "sysfs",
			Options:     []string{"nosuid", "noexec", "nodev", "ro"},
		},
		{
			Destination: "/sys/fs/cgroup",
			Type:        "cgroup",
			Source:      "cgroup",
			Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
		},
	},
	Linux: Linux{
		Resources: &Resources{
			Devices: []DeviceCgroup{
				{
					Allow:  false,
					Access: &rwm,
				},
			},
		},
		Namespaces: []Namespace{
			{
				Type: "pid",
			},
			{
				Type: "network",
			},
			{
				Type: "ipc",
			},
			{
				Type: "uts",
			},
			{
				Type: "mount",
			},
		},
	},
}

func BenchmarkMarshalSpec(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := ffjson.Marshal(spec)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var temp Spec
		if err := ffjson.Unmarshal(encoded, &temp); err != nil {
			b.Fatal(err)
		}
	}
}

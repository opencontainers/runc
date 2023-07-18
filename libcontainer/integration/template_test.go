package integration

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"golang.org/x/sys/unix"
)

var standardEnvironment = []string{
	"HOME=/root",
	"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	"HOSTNAME=integration",
	"TERM=xterm",
}

const defaultMountFlags = unix.MS_NOEXEC | unix.MS_NOSUID | unix.MS_NODEV

type tParam struct {
	userns  bool
	systemd bool
}

// newTemplateConfig returns a base template for running a container.
//
// It uses a network strategy of just setting a loopback interface
// and the default setup for devices.
//
// If p is nil, a default container is created.
func newTemplateConfig(t *testing.T, p *tParam) *configs.Config {
	var allowedDevices []*devices.Rule
	for _, device := range specconv.AllowedDevices {
		allowedDevices = append(allowedDevices, &device.Rule)
	}
	if p == nil {
		p = &tParam{}
	}
	config := &configs.Config{
		Rootfs: newRootfs(t),
		Capabilities: &configs.Capabilities{
			Bounding: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Permitted: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Ambient: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
			Effective: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
		},
		Namespaces: configs.Namespaces([]configs.Namespace{
			{Type: configs.NEWNS},
			{Type: configs.NEWUTS},
			{Type: configs.NEWIPC},
			{Type: configs.NEWPID},
			{Type: configs.NEWNET},
		}),
		Cgroups: &configs.Cgroup{
			Systemd: p.systemd,
			Resources: &configs.Resources{
				MemorySwappiness: nil,
				Devices:          allowedDevices,
			},
		},
		MaskPaths: []string{
			"/proc/kcore",
			"/sys/firmware",
		},
		ReadonlyPaths: []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		},
		Devices:    specconv.AllowedDevices,
		Hostname:   "integration",
		Domainname: "integration",
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
			/*
				            CI is broken on the debian based kernels with this
							{
								Source:      "mqueue",
								Destination: "/dev/mqueue",
								Device:      "mqueue",
								Flags:       defaultMountFlags,
							},
			*/
			{
				Source:      "sysfs",
				Destination: "/sys",
				Device:      "sysfs",
				Flags:       defaultMountFlags | unix.MS_RDONLY,
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

	if p.userns {
		config.UIDMappings = []configs.IDMap{{HostID: 0, ContainerID: 0, Size: 1000}}
		config.GIDMappings = []configs.IDMap{{HostID: 0, ContainerID: 0, Size: 1000}}
		config.Namespaces = append(config.Namespaces, configs.Namespace{Type: configs.NEWUSER})
	} else {
		config.Mounts = append(config.Mounts, &configs.Mount{
			Destination: "/sys/fs/cgroup",
			Device:      "cgroup",
			Flags:       defaultMountFlags | unix.MS_RDONLY,
		})
	}

	if p.systemd {
		id := strconv.FormatInt(-int64(time.Now().Nanosecond()), 36)
		config.Cgroups.Name = strings.ReplaceAll(t.Name(), "/", "_") + id
		config.Cgroups.Parent = "system.slice"
		config.Cgroups.ScopePrefix = "runc-test"
	} else {
		config.Cgroups.Path = "/test/integration"
	}

	return config
}

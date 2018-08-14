package specconv

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/opencontainers/runc/libcontainer/mount"
	"github.com/opencontainers/runtime-spec/specs-go"
)

// Example returns an example spec file, with many options set so a user can
// see what a standard spec file looks like.
func Example() *specs.Spec {
	return &specs.Spec{
		Version: specs.Version,
		Root: &specs.Root{
			Path:     "rootfs",
			Readonly: true,
		},
		Process: &specs.Process{
			Terminal: true,
			User:     specs.User{},
			Args: []string{
				"sh",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd:             "/",
			NoNewPrivileges: true,
			Capabilities: &specs.LinuxCapabilities{
				Bounding: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Permitted: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Inheritable: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Ambient: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Effective: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
			},
			Rlimits: []specs.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
		},
		Hostname: "runc",
		Mounts: []specs.Mount{
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
		Linux: &specs.Linux{
			MaskedPaths: []string{
				"/proc/kcore",
				"/proc/latency_stats",
				"/proc/timer_list",
				"/proc/timer_stats",
				"/proc/sched_debug",
				"/sys/firmware",
				"/proc/scsi",
			},
			ReadonlyPaths: []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
			Resources: &specs.LinuxResources{
				Devices: []specs.LinuxDeviceCgroup{
					{
						Allow:  false,
						Access: "rwm",
					},
				},
			},
			Namespaces: []specs.LinuxNamespace{
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
}

// ToRootless converts the given spec file into one that should work with
// rootless containers, by removing incompatible options and adding others that
// are needed.
func ToRootless(spec *specs.Spec) {
	var namespaces []specs.LinuxNamespace

	// Remove networkns from the spec.
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case specs.NetworkNamespace, specs.UserNamespace:
			// Do nothing.
		default:
			namespaces = append(namespaces, ns)
		}
	}
	// Add userns to the spec.
	namespaces = append(namespaces, specs.LinuxNamespace{
		Type: specs.UserNamespace,
	})
	spec.Linux.Namespaces = namespaces

	// Add mappings for the current user.
	spec.Linux.UIDMappings = []specs.LinuxIDMapping{{
		HostID:      uint32(os.Geteuid()),
		ContainerID: 0,
		Size:        1,
	}}
	spec.Linux.GIDMappings = []specs.LinuxIDMapping{{
		HostID:      uint32(os.Getegid()),
		ContainerID: 0,
		Size:        1,
	}}

	// Fix up mounts.
	mountInfo, _ := mount.GetMounts()
	fixupRootlessBindSys(spec, mountInfo)
	for i := range spec.Mounts {
		mount := &spec.Mounts[i]
		// Remove all gid= and uid= mappings.
		var options []string
		for _, option := range mount.Options {
			if !strings.HasPrefix(option, "gid=") && !strings.HasPrefix(option, "uid=") {
				options = append(options, option)
			}
		}
		mount.Options = options
	}

	// Remove cgroup settings.
	spec.Linux.Resources = nil
}

// fixupRootlessBindSys bind-mounts /sys.
// This is required only when netns is not unshared.
func fixupRootlessBindSys(spec *specs.Spec, mountInfo []*mount.Info) {
	// Fix up mounts.
	var (
		mounts         []specs.Mount
		mountsUnderSys []specs.Mount
	)
	for _, m := range spec.Mounts {
		// ignore original /sys (cannot be mounted when netns is not unshared)
		if filepath.Clean(m.Destination) == "/sys" {
			continue
		} else if strings.HasPrefix(m.Destination, "/sys") {
			// Append all mounts that are under /sys (e.g. /sys/fs/cgroup )later
			mountsUnderSys = append(mountsUnderSys, m)
		} else {
			mounts = append(mounts, m)
		}
	}
	mounts = append(mounts, []specs.Mount{
		// Add the sysfs mount as an rbind.
		// Note:
		// * "ro" does not make submounts read-only recursively.
		// * rbind work for sysfs but bind does not.
		{
			Source:      "/sys",
			Destination: "/sys",
			Type:        "none",
			Options:     []string{"rbind", "nosuid", "noexec", "nodev", "ro"},
		},
	}...)
	spec.Mounts = append(mounts, mountsUnderSys...)
	var (
		maskedPaths         []string
		maskedPathsWritable = make(map[string]struct{})
	)
	for _, masked := range spec.Linux.MaskedPaths {
		// e.g. when /sys/firmware is masked and /sys/firmware/efi/efivars is in /proc/self/mountinfo,
		// we need to mask /sys/firmware/efi/efivars as well.
		// (otherwise `df` fails with "df: /sys/firmware/efi/efivars: No such file or directory")
		// also, to mask /sys/firmware/efi/efivars, we need to mask /sys/firmware as a writable tmpfs
		if strings.HasPrefix(masked, "/sys") {
			for _, mi := range mountInfo {
				// e.g. mi.Mountpoint = /sys/firmware/efi/efivars, masked = /sys/firmware
				if strings.HasPrefix(mi.Mountpoint, masked) {
					maskedPathsWritable[masked] = struct{}{}
					// mi.Mountpoint is added to maskedPathsWritable for ease of supporting nested case
					maskedPathsWritable[mi.Mountpoint] = struct{}{}
				}
			}
		}
		if _, ok := maskedPathsWritable[masked]; !ok {
			maskedPaths = append(maskedPaths, masked)
		}
	}
	spec.Linux.MaskedPaths = maskedPaths
	for _, s := range sortMapKeyStrings(maskedPathsWritable) {
		spec.Mounts = append(spec.Mounts,
			specs.Mount{
				Source:      "none",
				Destination: s,
				Type:        "tmpfs",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=0755"},
			})
	}
}

func sortMapKeyStrings(m map[string]struct{}) []string {
	var ss []string
	for s := range m {
		ss = append(ss, s)
	}
	sort.Strings(ss)
	return ss
}

// +build linux,!no_systemd

package libcontainer

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func systemdCgroupV2(l *LinuxFactory, rootless bool) error {
	l.NewCgroupsManager = func(config *configs.Cgroup, paths map[string]string) cgroups.Manager {
		return systemd.NewUnifiedManager(config, getUnifiedPath(paths), rootless)
	}
	return nil
}

// SystemdCgroups is an options func to configure a LinuxFactory to return
// containers that use systemd to create and manage cgroups.
func SystemdCgroups(l *LinuxFactory) error {
	if !systemd.IsRunningSystemd() {
		return fmt.Errorf("systemd not running on this host, can't use systemd as cgroups manager")
	}

	if cgroups.IsCgroup2UnifiedMode() {
		return systemdCgroupV2(l, false)
	}

	l.NewCgroupsManager = func(config *configs.Cgroup, paths map[string]string) cgroups.Manager {
		return systemd.NewLegacyManager(config, paths)
	}

	return nil
}

// RootlessSystemdCgroups is rootless version of SystemdCgroups.
func RootlessSystemdCgroups(l *LinuxFactory) error {
	if !systemd.IsRunningSystemd() {
		return fmt.Errorf("systemd not running on this host, can't use systemd as cgroups manager")
	}

	if !cgroups.IsCgroup2UnifiedMode() {
		return fmt.Errorf("cgroup v2 not enabled on this host, can't use systemd (rootless) as cgroups manager")
	}
	return systemdCgroupV2(l, true)
}

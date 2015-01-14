package manager

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
)

// Create a new cgroup manager with specified configuration
func NewCgroupManager(cgroups *cgroups.Cgroup) cgroups.Manager {
	if systemd.UseSystemd() {
		return &systemd.Manager{
			Cgroups: cgroups,
		}
	}

	return &fs.Manager{
		Cgroups: cgroups,
	}
}

// Restore a cgroup manager with specified configuration and state
func LoadCgroupManager(cgroups *cgroups.Cgroup, paths map[string]string) cgroups.Manager {
	if systemd.UseSystemd() {
		return &systemd.Manager{
			Cgroups: cgroups,
			Paths:   paths,
		}
	}

	return &fs.Manager{
		Cgroups: cgroups,
		Paths:   paths,
	}
}

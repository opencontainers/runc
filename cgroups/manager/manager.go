package manager

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	"github.com/docker/libcontainer/configs"
)

// Create a new cgroup manager with specified configuration
// TODO this object is not really initialized until Apply() is called.
// Maybe make this to the equivalent of Apply() at some point?
// @vmarmol
func NewCgroupManager(cgroups *configs.Cgroup) cgroups.Manager {
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
func LoadCgroupManager(cgroups *configs.Cgroup, paths map[string]string) cgroups.Manager {
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

package manager

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
)

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

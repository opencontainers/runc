// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// NetPrioGroup represents net_prio control group.
type NetPrioGroup struct {
}

// Name returns the subsystem name of the cgroup.
func (s *NetPrioGroup) Name() string {
	return "net_prio"
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *NetPrioGroup) Apply(d *cgroupData) error {
	_, err := d.join("net_prio")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *NetPrioGroup) Set(path string, cgroup *configs.Cgroup) error {
	for _, prioMap := range cgroup.Resources.NetPrioIfpriomap {
		if err := writeFile(path, "net_prio.ifpriomap", prioMap.CgroupString()); err != nil {
			return err
		}
	}

	return nil
}

// Remove deletes the cgroup.
func (s *NetPrioGroup) Remove(d *cgroupData) error {
	return removePath(d.path("net_prio"))
}

// GetStats returns the statistic of the cgroup.
func (s *NetPrioGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

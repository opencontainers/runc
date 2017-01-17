// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// NameGroup represents name=systemd control group, it's not
// a real cgroup subsystem, but a cgroup used by systemd to
// manage cgroups.
type NameGroup struct {
	GroupName string
	Join      bool
}

// Name returns the group name of the cgroup.
func (s *NameGroup) Name() string {
	return s.GroupName
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *NameGroup) Apply(d *cgroupData) error {
	if s.Join {
		// ignore errors if the named cgroup does not exist
		d.join(s.GroupName)
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *NameGroup) Set(path string, cgroup *configs.Cgroup) error {
	return nil
}

// Remove deletes the cgroup.
func (s *NameGroup) Remove(d *cgroupData) error {
	if s.Join {
		removePath(d.path(s.GroupName))
	}
	return nil
}

// GetStats returns the statistic of the cgroup.
func (s *NameGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

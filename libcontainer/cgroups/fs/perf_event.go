// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// PerfEventGroup represents perf_event control group.
type PerfEventGroup struct {
}

// Name returns the subsystem name of the cgroup.
func (s *PerfEventGroup) Name() string {
	return "perf_event"
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *PerfEventGroup) Apply(d *cgroupData) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("perf_event"); err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *PerfEventGroup) Set(path string, cgroup *configs.Cgroup) error {
	return nil
}

// Remove deletes the cgroup.
func (s *PerfEventGroup) Remove(d *cgroupData) error {
	return removePath(d.path("perf_event"))
}

// GetStats returns the statistic of the cgroup.
func (s *PerfEventGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

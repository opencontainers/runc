// +build linux

package fs

import (
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// NetClsGroup represents net_cls control group.
type NetClsGroup struct {
}

// Name returns the subsystem name of the cgroup.
func (s *NetClsGroup) Name() string {
	return "net_cls"
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *NetClsGroup) Apply(d *cgroupData) error {
	_, err := d.join("net_cls")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *NetClsGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.NetClsClassid != 0 {
		if err := writeFile(path, "net_cls.classid", strconv.FormatUint(uint64(cgroup.Resources.NetClsClassid), 10)); err != nil {
			return err
		}
	}

	return nil
}

// Remove deletes the cgroup.
func (s *NetClsGroup) Remove(d *cgroupData) error {
	return removePath(d.path("net_cls"))
}

// GetStats returns the statistic of the cgroup.
func (s *NetClsGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

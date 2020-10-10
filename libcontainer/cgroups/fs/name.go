// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type NameGroup struct {
	GroupName string
	Join      bool
}

func (s *NameGroup) Name() string {
	return s.GroupName
}

func (s *NameGroup) Apply(path string, d *cgroupData) (retErr error) {
	if s.Join {
		// ignore errors if the named cgroup does not exist
		retErr = join(path, d.pid)
	}
	return
}

func (s *NameGroup) Set(path string, cgroup *configs.Cgroup) error {
	return nil
}

func (s *NameGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

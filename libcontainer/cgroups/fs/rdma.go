// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type RdmaGroup struct{}

func (s *RdmaGroup) Name() string {
	return "rdma"
}

func (s *RdmaGroup) Apply(path string, d *cgroupData) error {
	return join(path, d.pid)
}

func (s *RdmaGroup) Set(path string, r *configs.Resources) error {
	return fscommon.RdmaSet(path, r)
}

func (s *RdmaGroup) GetStats(path string, stats *cgroups.Stats) error {
	return fscommon.RdmaGetStats(path, stats)
}

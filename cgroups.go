package libcontainer

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
)

type CgroupManager interface {
	GetPids(*cgroups.Cgroup) ([]int, error)
	GetStats(*cgroups.Cgroup) (*cgroups.Stats, error)
}

func newCgroupsManager() CgroupManager {
	if systemd.UseSystemd() {
		return &systemdCgroupManager{}
	}
	return &fsCgroupsManager{}
}

type systemdCgroupManager struct {
}

func (m *systemdCgroupManager) GetPids(config *cgroups.Cgroup) ([]int, error) {
	return systemd.GetPids(config)
}

func (m *systemdCgroupManager) GetStats(config *cgroups.Cgroup) (*cgroups.Stats, error) {
	return systemd.GetStats(config)
}

type fsCgroupsManager struct {
}

func (m *fsCgroupsManager) GetPids(config *cgroups.Cgroup) ([]int, error) {
	return fs.GetPids(config)
}

func (m *fsCgroupsManager) GetStats(config *cgroups.Cgroup) (*cgroups.Stats, error) {
	return fs.GetStats(config)
}

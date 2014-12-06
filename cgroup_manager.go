package libcontainer

import (
	"github.com/docker/libcontainer/cgroups"
)

// TODO(vmarmol): Move this to cgroups and rename to Manager.
type CgroupManager interface {
	GetPids() ([]int, error)
	GetStats() (*cgroups.Stats, error)
}

func NewCgroupManager() CgroupManager {
	return &fsManager{}
}

type fsManager struct {
}

func (m *fsManager) GetPids() ([]int, error) {
	// TODO(vmarmol): Implement
	//return fs.GetPids(config)
	panic("not implemented")
}

func (m *fsManager) GetStats() (*cgroups.Stats, error) {
	// TODO(vmarmol): Implement
	//return fs.GetStats(config)
	panic("not implemented")
}

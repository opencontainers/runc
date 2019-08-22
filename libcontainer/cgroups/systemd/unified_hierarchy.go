// +build linux,!static_build

package systemd

import (
	"fmt"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

// IsCgroup2UnifiedMode returns whether we are running in cgroup v2 unified mode.
func IsCgroup2UnifiedMode() (bool, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/sys/fs/cgroup", &st); err != nil {
		return false, fmt.Errorf("cannot statfs cgroup root: %v", err)
	}
	return st.Type == unix.CGROUP2_SUPER_MAGIC, nil
}

type UnifiedManager struct {
	Cgroups *configs.Cgroup
	Paths   map[string]string
}

func NewUnifiedSystemdCgroupsManager() (func(config *configs.Cgroup, paths map[string]string) cgroups.Manager, error) {
	return nil, fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) Apply(pid int) error {
	return fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) GetPids() ([]int, error) {
	return nil, fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) GetAllPids() ([]int, error) {
	return nil, fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) Destroy() error {
	return fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) GetPaths() map[string]string {
	return nil
}

func (m *UnifiedManager) GetStats() (*cgroups.Stats, error) {
	return nil, fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) Set(container *configs.Config) error {
	return fmt.Errorf("unified hierarchy not supported")
}

func (m *UnifiedManager) Freeze(state configs.FreezerState) error {
	return fmt.Errorf("unified hierarchy not supported")
}

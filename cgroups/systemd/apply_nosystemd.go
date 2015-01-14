// +build !linux

package systemd

import (
	"fmt"

	"github.com/docker/libcontainer/cgroups"
)

type Manager struct {
	Cgroups *cgroups.Cgroup
	Paths   map[string]string
}

func UseSystemd() bool {
	return false
}

func (m *Manager) Apply(pid int) error {
	return fmt.Errorf("Systemd not supported")
}

func (m *Manager) GetPids() ([]int, error) {
	return nil, fmt.Errorf("Systemd not supported")
}

func (m *Manager) RemovePaths() error {
	return fmt.Errorf("Systemd not supported")
}

func (m *Manager) GetPaths() map[string]string {
	return nil
}

func (m *Manager) GetStats() (*cgroups.Stats, error) {
	return nil, fmt.Errorf("Systemd not supported")
}

func (m *Manager) Freeze(state cgroups.FreezerState) error {
	return fmt.Errorf("Systemd not supported")
}

func ApplyDevices(c *cgroups.Cgroup, pid int) error {
	return fmt.Errorf("Systemd not supported")
}

func Freeze(c *cgroups.Cgroup, state cgroups.FreezerState) error {
	return fmt.Errorf("Systemd not supported")
}

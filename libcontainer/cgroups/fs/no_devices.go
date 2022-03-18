//go:build disable_cgroup_devices
// +build disable_cgroup_devices

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func skipDevices(_ *configs.Resources) bool {
	return true
}

type DevicesGroup struct{}

func (s *DevicesGroup) Name() string {
	return "devices"
}

func (s *DevicesGroup) Apply(_ string, _ *configs.Resources, _ int) error {
	return nil
}

func (s *DevicesGroup) Set(_ string, _ *configs.Resources) error {
	return nil
}

func (s *DevicesGroup) GetStats(_ string, _ *cgroups.Stats) error {
	return nil
}

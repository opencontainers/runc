//go:build disable_cgroup_devices
// +build disable_cgroup_devices

package fs2

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

func setDevices(_ string, _ *configs.Resources) error {
	return nil
}

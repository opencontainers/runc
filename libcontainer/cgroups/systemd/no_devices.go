//go:build disable_cgroup_devices
// +build disable_cgroup_devices

package systemd

import (
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func (_ *legacyManager) freezeBeforeSet(_ string, _ *configs.Resources) (needsFreeze, needsThaw bool, err error) {
	return false, false, nil
}

func generateDeviceProperties(_ *configs.Resources) ([]systemdDbus.Property, error) {
	return nil, nil
}

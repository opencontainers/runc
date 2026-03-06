//go:build !windows

// Deprecated: Use github.com/moby/sys/devices instead. This package will be
// removed in runc 1.7.
package devices

import (
	"github.com/moby/sys/devices"
	"github.com/opencontainers/cgroups/devices/config"
)

// ErrNotADevice denotes that a file is not a valid linux device.
//
// Deprecated: Use [devices.ErrNotADevice] instead. This will be removed in
// runc 1.7.
var ErrNotADevice = devices.ErrNotADevice

// DeviceFromPath takes the path to a device and its cgroup_permissions (which
// cannot be easily queried) to look up the information about a linux device
// and returns that information as a Device struct.
//
// Deprecated: Use [devices.DeviceFromPath] instead. This will be removed in
// runc 1.7.
func DeviceFromPath(path, permissions string) (*config.Device, error) {
	return devices.DeviceFromPath(path, permissions)
}

// HostDevices returns all devices that can be found under /dev directory.
//
// Deprecated: Use [devices.HostDevices] instead. This will be removed in runc
// 1.7.
func HostDevices() ([]*config.Device, error) {
	return devices.HostDevices()
}

// GetDevices recursively traverses a directory specified by path
// and returns all devices found there.
//
// Deprecated: Use [devices.GetDevices] instead. This will be removed in runc
// 1.7.
func GetDevices(path string) ([]*config.Device, error) {
	return devices.GetDevices(path)
}

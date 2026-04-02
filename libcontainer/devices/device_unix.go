//go:build !windows

package devices

import (
	"github.com/moby/sys/devices"
	"github.com/opencontainers/cgroups/devices/config"
)

// ErrNotADevice denotes that a file is not a valid linux device.
//
// Deprecated: This package will be removed in runc 1.7, use
// [devices.ErrNotADevice] instead.
var ErrNotADevice = devices.ErrNotADevice

// DeviceFromPath takes the path to a device and its cgroup_permissions (which
// cannot be easily queried) to look up the information about a linux device
// and returns that information as a Device struct.
//
// Deprecated: This package will be removed in runc 1.7, use
// [devices.DeviceFromPath] instead.
func DeviceFromPath(path, permissions string) (*config.Device, error) {
	return devices.DeviceFromPath(path, permissions)
}

// HostDevices returns all devices that can be found under /dev directory.
//
// Deprecated: This package will be removed in runc 1.7, use
// [devices.HostDevices] instead.
func HostDevices() ([]*config.Device, error) {
	return devices.HostDevices()
}

// GetDevices recursively traverses a directory specified by path
// and returns all devices found there.
//
// Deprecated: This package will be removed in runc 1.7, use
// [devices.GetDevices] instead.
func GetDevices(path string) ([]*config.Device, error) {
	return devices.GetDevices(path)
}

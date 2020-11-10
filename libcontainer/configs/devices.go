package configs

import "github.com/opencontainers/runc/libcontainer/devices"

type (
	// Deprecated: use libcontainer/devices.Device
	Device = devices.Device

	// Deprecated: use libcontainer/devices.DeviceRule
	DeviceRule = devices.DeviceRule

	// Deprecated: use libcontainer/devices.DeviceType
	DeviceType = devices.DeviceType

	// Deprecated: use libcontainer/devices.DevicePermissions
	DevicePermissions = devices.DevicePermissions
)

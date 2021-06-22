// +build linux,!no_systemd

package configs

import (
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
)

// Systemd specific parts of Cgroup properties
type CgroupSD struct {
	// SystemdProps are any additional properties for systemd,
	// derived from org.systemd.property.xxx annotations.
	// Ignored unless systemd is used for managing cgroups.
	SystemdProps []systemdDbus.Property `json:"-"`
}

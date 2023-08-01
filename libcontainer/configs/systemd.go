//go:build linux && !no_systemd

package configs

import (
	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
)

type (
	SdProperty   = systemdDbus.Property
	SdProperties = []systemdDbus.Property
)

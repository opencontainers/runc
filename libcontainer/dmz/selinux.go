//go:build runc_dmz_selinux_nocompat || !linux

package dmz

import "github.com/opencontainers/runc/libcontainer/configs"

// WorksWithSELinux tells whether runc-dmz can work with SELinux.
func WorksWithSELinux(*configs.Config) bool {
	return true
}

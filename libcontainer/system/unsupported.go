// +build !linux

package system

import (
	"github.com/opencontainers/runc/libcontainer/user"
)

// RunningInUserNS is a stub for non-Linux systems
// Always returns false
func RunningInUserNS() bool {
	return false
}

// uidMapInUserNS is a stub for non-Linux systems
// Always returns false
func uidMapInUserNS(uidmap []user.IDMap) bool {
	return false
}

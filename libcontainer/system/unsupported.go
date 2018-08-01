// +build !linux

package system

import (
	"os"

	"github.com/opencontainers/runc/libcontainer/user"
)

// RunningInUserNS is a stub for non-Linux systems
// Always returns false
func RunningInUserNS() bool {
	return false
}

// UIDMapInUserNS is a stub for non-Linux systems
// Always returns false
func UIDMapInUserNS(uidmap []user.IDMap) bool {
	return false
}

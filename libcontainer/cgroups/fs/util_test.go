// +build linux

/*
Utility for testing cgroup operations.

Creates a mock of the cgroup filesystem for the duration of the test.
*/
package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func init() {
	cgroups.TestMode = true
}

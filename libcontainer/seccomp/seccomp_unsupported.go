// +build !linux !cgo

package seccomp

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Seccomp not supported, do nothing
func InitSeccomp(config *configs.Seccomp) error {
	return nil
}

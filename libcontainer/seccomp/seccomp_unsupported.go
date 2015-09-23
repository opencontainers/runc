// +build !linux !cgo !seccomp

package seccomp

import (
	"github.com/opencontainers/runc/libcontainer/configs"
)

// Seccomp not supported, do nothing
func InitSeccomp(config *configs.Seccomp) error {
	if config != nil {
		return ErrSeccompNotEnabled
	}
	return nil
}

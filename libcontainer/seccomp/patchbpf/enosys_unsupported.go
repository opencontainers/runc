// +build !linux !cgo !seccomp

package patchbpf

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"

	libseccomp "github.com/seccomp/libseccomp-golang"
)

func PatchAndLoad(config *configs.Seccomp, filter *libseccomp.ScmpFilter) error {
	if config != nil {
		return errors.New("cannot patch and load seccomp filter without runc seccomp support")
	}
	return nil
}

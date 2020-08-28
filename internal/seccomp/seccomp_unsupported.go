// +build !linux !cgo !seccomp

package seccomp

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// ErrSeccompNotEnabled will be returned if seccomp is build-time disabled in runc
var ErrSeccompNotEnabled = errors.New("config provided but seccomp not supported")

// Setup does nothing because seccomp is not supported.
func Setup(spec *specs.LinuxSeccomp) error {
	if spec != nil {
		return ErrSeccompNotEnabled
	}
	return nil
}

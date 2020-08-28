// +build linux,cgo,seccomp

package seccomp

import (
	"github.com/containers/common/pkg/seccomp"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Setup takes the provided spec and loads the built seccomp filter into the
// kernel.
func Setup(spec *specs.LinuxSeccomp) error {
	filter, err := seccomp.BuildFilter(spec)
	if err != nil {
		return errors.Wrap(err, "build filter")
	}
	if err := filter.Load(); err != nil {
		return errors.Wrap(err, "load filter")
	}
	return nil
}

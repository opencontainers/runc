// +build linux,no_systemd

package specconv

import (
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func initSystemdCgroups(opts *CreateOpts, spec *specs.Spec, c *configs.Cgroup) (error) {
	return initPlainCgroups(opts, spec, c)
}

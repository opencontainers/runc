//go:build linux && runc_nosd

package specconv

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func initSystemdProps(spec *specs.Spec) (configs.SdProperties, error) {
	var sp configs.SdProperties

	return sp, nil
}

func createCgroupConfigSystemd(opts *CreateOpts, c *configs.Cgroup) error {
	return errors.New("no systemd support")
}

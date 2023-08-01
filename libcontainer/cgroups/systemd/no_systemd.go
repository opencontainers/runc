//go:build linux && runc_nosd

package systemd

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func IsRunningSystemd() bool {
	return false
}

func NewUnifiedManager(config *configs.Cgroup, path string) (cgroups.Manager, error) {
	return nil, errors.New("no systemd support")
}

func NewLegacyManager(cg *configs.Cgroup, paths map[string]string) (cgroups.Manager, error) {
	return nil, errors.New("no systemd support")
}

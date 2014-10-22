// +build linux

package libcontainer

import (
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/cgroups/systemd"
	"github.com/docker/libcontainer/network"
)

type linuxContainer struct {
	id     string
	root   string
	config *Config
	state  *State
}

func (c *linuxContainer) ID() string {
	return c.id
}

func (c *linuxContainer) Config() *Config {
	return c.config
}

func (c *linuxContainer) RunState() (*RunState, Error) {
	panic("not implemented")
}

func (c *linuxContainer) Processes() ([]int, Error) {
	var (
		err  error
		pids []int
	)

	if systemd.UseSystemd() {
		pids, err = systemd.GetPids(c.config.Cgroups)
	} else {
		pids, err = fs.GetPids(c.config.Cgroups)
	}
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return pids, nil
}

func (c *linuxContainer) Stats() (*ContainerStats, Error) {
	var (
		err   error
		stats = &ContainerStats{}
	)

	if systemd.UseSystemd() {
		stats.CgroupStats, err = systemd.GetStats(c.config.Cgroups)
	} else {
		stats.CgroupStats, err = fs.GetStats(c.config.Cgroups)
	}
	if err != nil {
		return stats, newGenericError(err, SystemError)
	}

	if stats.NetworkStats, err = network.GetStats(&c.state.NetworkState); err != nil {
		return stats, newGenericError(err, SystemError)
	}
	return stats, nil
}

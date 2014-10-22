// +build linux

package libcontainer

import "github.com/docker/libcontainer/network"

type linuxContainer struct {
	id            string
	root          string
	config        *Config
	state         *State
	cgroupManager CgroupManager
}

func (c *linuxContainer) ID() string {
	return c.id
}

func (c *linuxContainer) Config() *Config {
	return c.config
}

func (c *linuxContainer) RunState() (RunState, error) {
	panic("not implemented")
}

func (c *linuxContainer) Processes() ([]int, error) {
	pids, err := c.cgroupManager.GetPids(c.config.Cgroups)
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return pids, nil
}

func (c *linuxContainer) Stats() (*ContainerStats, error) {
	var (
		err   error
		stats = &ContainerStats{}
	)

	if stats.CgroupStats, err = c.cgroupManager.GetStats(c.config.Cgroups); err != nil {
		return stats, newGenericError(err, SystemError)
	}
	if stats.NetworkStats, err = network.GetStats(&c.state.NetworkState); err != nil {
		return stats, newGenericError(err, SystemError)
	}
	return stats, nil
}

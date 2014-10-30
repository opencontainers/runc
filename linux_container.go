// +build linux

package libcontainer

import (
	"github.com/Sirupsen/logrus"
	"github.com/docker/libcontainer/network"
)

type linuxContainer struct {
	id            string
	root          string
	config        *Config
	state         *State
	cgroupManager CgroupManager
	logger        *logrus.Logger
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
	c.logger.Debug("fetch container processes")
	pids, err := c.cgroupManager.GetPids(c.config.Cgroups)
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return pids, nil
}

func (c *linuxContainer) Stats() (*ContainerStats, error) {
	c.logger.Debug("fetch container stats")
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

func (c *linuxContainer) StartProcess(config *ProcessConfig) (int, error) {
	c.logger.Debug("start new container process")
	panic("not implemented")
}

func (c *linuxContainer) Destroy() error {
	c.logger.Debug("destroy container")
	panic("not implemented")
}

func (c *linuxContainer) Pause() error {
	c.logger.Debug("pause container")
	panic("not implemented")
}

func (c *linuxContainer) Resume() error {
	c.logger.Debug("resume container")
	panic("not implemented")
}

func (c *linuxContainer) Signal(pid, signal int) error {
	c.logger.Debugf("sending signal %d to pid %d", signal, pid)
	panic("not implemented")
}

func (c *linuxContainer) Wait() (int, error) {
	c.logger.Debug("wait container")
	panic("not implemented")
}

func (c *linuxContainer) WaitProcess(pid int) (int, error) {
	c.logger.Debugf("wait process %d", pid)
	panic("not implemented")
}

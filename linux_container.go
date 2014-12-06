// +build linux

package libcontainer

import (
	"github.com/docker/libcontainer/network"
	"github.com/golang/glog"
)

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
	glog.Info("fetch container processes")
	pids, err := c.cgroupManager.GetPids()
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return pids, nil
}

func (c *linuxContainer) Stats() (*ContainerStats, error) {
	glog.Info("fetch container stats")
	var (
		err   error
		stats = &ContainerStats{}
	)

	if stats.CgroupStats, err = c.cgroupManager.GetStats(); err != nil {
		return stats, newGenericError(err, SystemError)
	}
	if stats.NetworkStats, err = network.GetStats(&c.state.NetworkState); err != nil {
		return stats, newGenericError(err, SystemError)
	}
	return stats, nil
}

func (c *linuxContainer) StartProcess(config *ProcessConfig) (int, error) {
	glog.Info("start new container process")
	panic("not implemented")
}

func (c *linuxContainer) Destroy() error {
	glog.Info("destroy container")
	panic("not implemented")
}

func (c *linuxContainer) Pause() error {
	glog.Info("pause container")
	panic("not implemented")
}

func (c *linuxContainer) Resume() error {
	glog.Info("resume container")
	panic("not implemented")
}

func (c *linuxContainer) Signal(pid, signal int) error {
	glog.Infof("sending signal %d to pid %d", signal, pid)
	panic("not implemented")
}

func (c *linuxContainer) Wait() (int, error) {
	glog.Info("wait container")
	panic("not implemented")
}

func (c *linuxContainer) WaitProcess(pid int) (int, error) {
	glog.Infof("wait process %d", pid)
	panic("not implemented")
}

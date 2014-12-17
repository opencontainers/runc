// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/network"
	"github.com/golang/glog"
)

type linuxContainer struct {
	id            string
	root          string
	config        *configs.Config
	state         *configs.State
	cgroupManager CgroupManager
	initArgs      []string
}

func (c *linuxContainer) ID() string {
	return c.id
}

func (c *linuxContainer) Config() *configs.Config {
	return c.config
}

func (c *linuxContainer) RunState() (configs.RunState, error) {
	return configs.Destroyed, nil // FIXME return a real state
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

func (c *linuxContainer) StartProcess(pconfig *ProcessConfig) (int, error) {
	state, err := c.RunState()
	if err != nil {
		return -1, err
	}

	if state != configs.Destroyed {
		glog.Info("start new container process")
		panic("not implemented")
	}

	if err := c.startInitProcess(pconfig); err != nil {
		return -1, err
	}

	return c.state.InitPid, nil
}

func (c *linuxContainer) updateStateFile() error {
	data, err := json.MarshalIndent(c.state, "", "\t")
	if err != nil {
		return newGenericError(err, SystemError)
	}

	fnew := filepath.Join(c.root, fmt.Sprintf("%s.new", stateFilename))
	f, err := os.Create(fnew)
	if err != nil {
		return newGenericError(err, SystemError)
	}

	_, err = f.Write(data)
	if err != nil {
		f.Close()
		return newGenericError(err, SystemError)
	}
	f.Close()

	fname := filepath.Join(c.root, stateFilename)
	if err := os.Rename(fnew, fname); err != nil {
		return newGenericError(err, SystemError)
	}

	return nil
}

func (c *linuxContainer) startInitProcess(config *ProcessConfig) error {
	cmd := exec.Command(c.initArgs[0], append(c.initArgs[1:], config.Args...)...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr

	cmd.Env = config.Env
	cmd.Dir = c.config.RootFs

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL

	//FIXME call namespaces.Exec()
	if err := cmd.Start(); err != nil {
		return err
	}

	c.state.InitPid = cmd.Process.Pid
	err := c.updateStateFile()
	if err != nil {
		return err
	}

	return nil
}

func (c *linuxContainer) Destroy() error {
	state, err := c.RunState()
	if err != nil {
		return err
	}

	if state != configs.Destroyed {
		return newGenericError(nil, ContainerNotStopped)
	}

	os.RemoveAll(c.root)
	return nil
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

// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/namespaces"
	"github.com/docker/libcontainer/network"
	"github.com/golang/glog"
)

type linuxContainer struct {
	id            string
	root          string
	config        *configs.Config
	state         *configs.State
	cgroupManager cgroups.Manager
	initArgs      []string
}

func (c *linuxContainer) ID() string {
	return c.id
}

func (c *linuxContainer) Config() *configs.Config {
	return c.config
}

func (c *linuxContainer) Status() (configs.Status, error) {
	if c.state.InitPid <= 0 {
		return configs.Destroyed, nil
	}
	// return Running if the init process is alive
	err := syscall.Kill(c.state.InitPid, 0)
	if err != nil {
		if err == syscall.ESRCH {
			return configs.Destroyed, nil
		}
		return 0, err
	}
	if c.config.Cgroups != nil &&
		c.config.Cgroups.Freezer == configs.Frozen {
		return configs.Paused, nil
	}
	return configs.Running, nil
}

func (c *linuxContainer) Processes() ([]int, error) {
	glog.Info("fetch container processes")
	pids, err := c.cgroupManager.GetPids()
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return pids, nil
}

func (c *linuxContainer) Stats() (*Stats, error) {
	glog.Info("fetch container stats")
	var (
		err   error
		stats = &Stats{}
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
	status, err := c.Status()
	if err != nil {
		return -1, err
	}

	cmd := exec.Command(c.initArgs[0], c.initArgs[1:]...)
	cmd.Stdin = config.Stdin
	cmd.Stdout = config.Stdout
	cmd.Stderr = config.Stderr

	cmd.Env = config.Env
	cmd.Dir = c.config.RootFs

	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}

	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL

	if status != configs.Destroyed {
		glog.Info("start new container process")
		return namespaces.ExecIn(config.Args, config.Env, config.Console, cmd, c.config, c.state)
	}
	if err := c.startInitProcess(cmd, config); err != nil {
		return -1, err
	}
	return c.state.InitPid, nil
}

func (c *linuxContainer) updateStateFile() error {
	fnew := filepath.Join(c.root, fmt.Sprintf("%s.new", stateFilename))
	f, err := os.Create(fnew)
	if err != nil {
		return newGenericError(err, SystemError)
	}

	err = json.NewEncoder(f).Encode(c.state)
	if err != nil {
		f.Close()
		os.Remove(fnew)
		return newGenericError(err, SystemError)
	}
	f.Close()

	fname := filepath.Join(c.root, stateFilename)
	if err := os.Rename(fnew, fname); err != nil {
		return newGenericError(err, SystemError)
	}

	return nil
}

func (c *linuxContainer) startInitProcess(cmd *exec.Cmd, config *ProcessConfig) error {
	err := namespaces.Exec(config.Args, config.Env, config.Console, cmd, c.config, c.cgroupManager, c.state)
	if err != nil {
		return err
	}

	err = c.updateStateFile()
	if err != nil {
		// FIXME c.Kill()
		return err
	}

	return nil
}

func (c *linuxContainer) Destroy() error {
	status, err := c.Status()
	if err != nil {
		return err
	}
	if status != configs.Destroyed {
		return newGenericError(nil, ContainerNotStopped)
	}
	return os.RemoveAll(c.root)
}

func (c *linuxContainer) Pause() error {
	return c.cgroupManager.Freeze(configs.Frozen)
}

func (c *linuxContainer) Resume() error {
	return c.cgroupManager.Freeze(configs.Thawed)
}

func (c *linuxContainer) Signal(pid, signal int) error {
	glog.Infof("sending signal %d to pid %d", signal, pid)
	panic("not implemented")
}

func (c *linuxContainer) Wait() (int, error) {
	return c.WaitProcess(c.state.InitPid)
}

func (c *linuxContainer) WaitProcess(pid int) (int, error) {
	var status syscall.WaitStatus

	_, err := syscall.Wait4(pid, &status, 0, nil)
	if err != nil {
		return -1, newGenericError(err, SystemError)
	}

	return int(status), err
}

func (c *linuxContainer) OOM() (<-chan struct{}, error) {
	return NotifyOnOOM(c.state)
}

// +build linux

package libcontainer

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/golang/glog"
)

type linuxContainer struct {
	id            string
	root          string
	config        *configs.Config
	cgroupManager cgroups.Manager
	initArgs      []string
	initProcess   parentProcess
}

// ID returns the container's unique ID
func (c *linuxContainer) ID() string {
	return c.id
}

// Config returns the container's configuration
func (c *linuxContainer) Config() configs.Config {
	return *c.config
}

func (c *linuxContainer) Status() (configs.Status, error) {
	if c.initProcess == nil {
		return configs.Destroyed, nil
	}
	// return Running if the init process is alive
	if err := syscall.Kill(c.initProcess.pid(), 0); err != nil {
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
	for _, iface := range c.config.Networks {
		switch iface.Type {
		case "veth":
			istats, err := getNetworkInterfaceStats(iface.VethHost)
			if err != nil {
				return stats, newGenericError(err, SystemError)
			}
			stats.Interfaces = append(stats.Interfaces, istats)
		}
	}
	return stats, nil
}

func (c *linuxContainer) Start(process *Process) (int, error) {
	status, err := c.Status()
	if err != nil {
		return -1, err
	}
	doInit := status == configs.Destroyed
	parent, err := c.newParentProcess(process, doInit)
	if err != nil {
		return -1, err
	}
	if err := parent.start(); err != nil {
		// terminate the process to ensure that it properly is reaped.
		if err := parent.terminate(); err != nil {
			glog.Warning(err)
		}
		return -1, err
	}
	if doInit {
		c.initProcess = parent
	}
	return parent.pid(), nil
}

func (c *linuxContainer) newParentProcess(p *Process, doInit bool) (parentProcess, error) {
	parentPipe, childPipe, err := newPipe()
	if err != nil {
		return nil, err
	}
	cmd, err := c.commandTemplate(p, childPipe)
	if err != nil {
		return nil, err
	}
	if !doInit {
		return c.newSetnsProcess(p, cmd, parentPipe, childPipe), nil
	}
	return c.newInitProcess(p, cmd, parentPipe, childPipe), nil
}

func (c *linuxContainer) commandTemplate(p *Process, childPipe *os.File) (*exec.Cmd, error) {
	cmd := exec.Command(c.initArgs[0], c.initArgs[1:]...)
	cmd.Stdin = p.Stdin
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Dir = c.config.Rootfs
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.ExtraFiles = []*os.File{childPipe}
	cmd.SysProcAttr.Pdeathsig = syscall.Signal(c.config.ParentDeathSignal)
	return cmd, nil
}

func (c *linuxContainer) newInitProcess(p *Process, cmd *exec.Cmd, parentPipe, childPipe *os.File) *initProcess {
	cloneFlags := c.config.Namespaces.CloneFlags()
	if cloneFlags&syscall.CLONE_NEWUSER != 0 {
		c.addUidGidMappings(cmd.SysProcAttr)
		// Default to root user when user namespaces are enabled.
		if cmd.SysProcAttr.Credential == nil {
			cmd.SysProcAttr.Credential = &syscall.Credential{}
		}
	}
	cmd.SysProcAttr.Cloneflags = cloneFlags
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE=standard")
	return &initProcess{
		cmd:        cmd,
		childPipe:  childPipe,
		parentPipe: parentPipe,
		manager:    c.cgroupManager,
		config:     c.newInitConfig(p),
	}
}

func (c *linuxContainer) newSetnsProcess(p *Process, cmd *exec.Cmd, parentPipe, childPipe *os.File) *setnsProcess {
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("_LIBCONTAINER_INITPID=%d", c.initProcess.pid()),
		"_LIBCONTAINER_INITTYPE=setns",
	)
	// TODO: set on container for process management
	return &setnsProcess{
		cmd:         cmd,
		cgroupPaths: c.cgroupManager.GetPaths(),
		childPipe:   childPipe,
		parentPipe:  parentPipe,
		config:      c.newInitConfig(p),
	}
}

func (c *linuxContainer) newInitConfig(process *Process) *initConfig {
	return &initConfig{
		Config: c.config,
		Args:   process.Args,
		Env:    process.Env,
	}
}

// Converts IDMap to SysProcIDMap array and adds it to SysProcAttr.
func (c *linuxContainer) addUidGidMappings(sys *syscall.SysProcAttr) {
	if c.config.UidMappings != nil {
		sys.UidMappings = make([]syscall.SysProcIDMap, len(c.config.UidMappings))
		for i, um := range c.config.UidMappings {
			sys.UidMappings[i].ContainerID = um.ContainerID
			sys.UidMappings[i].HostID = um.HostID
			sys.UidMappings[i].Size = um.Size
		}
	}
	if c.config.GidMappings != nil {
		sys.GidMappings = make([]syscall.SysProcIDMap, len(c.config.GidMappings))
		for i, gm := range c.config.GidMappings {
			sys.GidMappings[i].ContainerID = gm.ContainerID
			sys.GidMappings[i].HostID = gm.HostID
			sys.GidMappings[i].Size = gm.Size
		}
	}
}

func newPipe() (parent *os.File, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

func (c *linuxContainer) Destroy() error {
	status, err := c.Status()
	if err != nil {
		return err
	}
	if status != configs.Destroyed {
		return newGenericError(nil, ContainerNotStopped)
	}
	// TODO: remove cgroups
	return os.RemoveAll(c.root)
}

func (c *linuxContainer) Pause() error {
	return c.cgroupManager.Freeze(configs.Frozen)
}

func (c *linuxContainer) Resume() error {
	return c.cgroupManager.Freeze(configs.Thawed)
}

func (c *linuxContainer) Signal(signal os.Signal) error {
	glog.Infof("sending signal %d to pid %d", signal, c.initProcess.pid())
	return c.initProcess.signal(signal)
}

// TODO: rename to be more descriptive
func (c *linuxContainer) OOM() (<-chan struct{}, error) {
	return NotifyOnOOM(c.cgroupManager.GetPaths())
}

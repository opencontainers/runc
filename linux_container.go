// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/system"
	"github.com/golang/glog"
)

const (
	EXIT_SIGNAL_OFFSET = 128
)

type pid struct {
	Pid int `json:"Pid"`
}

type linuxContainer struct {
	id            string
	root          string
	config        *configs.Config
	state         *configs.State
	cgroupManager cgroups.Manager
	initArgs      []string
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

func (c *linuxContainer) Start(process *Process) (int, error) {
	status, err := c.Status()
	if err != nil {
		return -1, err
	}
	cmd := c.commandTemplate(process)
	if status != configs.Destroyed {
		// TODO: (crosbymichael) check out console use for execin
		return c.startNewProcess(cmd, process.Args)
		//return namespaces.ExecIn(process.Args, c.config.Env, "", cmd, c.config, c.state)
	}
	if err := c.startInitialProcess(cmd, process.Args); err != nil {
		return -1, err
	}
	return c.state.InitPid, nil
}

// commandTemplate creates a template *exec.Cmd.  It uses the init arguments provided
// to the factory and attaches IO to the process.
func (c *linuxContainer) commandTemplate(process *Process) *exec.Cmd {
	cmd := exec.Command(c.initArgs[0], c.initArgs[1:]...)
	cmd.Stdin = process.Stdin
	cmd.Stdout = process.Stdout
	cmd.Stderr = process.Stderr
	cmd.Env = c.config.Env
	cmd.Dir = c.config.Rootfs
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// TODO: add pdeath to config for a container
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
	return cmd
}

// startNewProcess adds another process to an already running container
func (c *linuxContainer) startNewProcess(cmd *exec.Cmd, args []string) (int, error) {
	glog.Info("start new container process")
	parent, child, err := newInitPipe()
	if err != nil {
		return -1, err
	}
	defer parent.Close()
	cmd.ExtraFiles = []*os.File{child}
	cmd.Env = append(cmd.Env, fmt.Sprintf("_LIBCONTAINER_INITPID=%d", c.state.InitPid), "_LIBCONTAINER_INITTYPE=setns")

	// start the command
	err = cmd.Start()
	child.Close()
	if err != nil {
		return -1, err
	}
	s, err := cmd.Process.Wait()
	if err != nil {
		return -1, err
	}
	if !s.Success() {
		return -1, &exec.ExitError{s}
	}
	decoder := json.NewDecoder(parent)
	var pid *pid
	if err := decoder.Decode(&pid); err != nil {
		return -1, err
	}
	p, err := os.FindProcess(pid.Pid)
	if err != nil {
		return -1, err
	}
	terminate := func(terr error) (int, error) {
		// TODO: log the errors for kill and wait
		if err := p.Kill(); err != nil {
			glog.Warning(err)
		}
		if _, err := p.Wait(); err != nil {
			glog.Warning(err)
		}
		return -1, terr
	}
	if err := c.enterCgroups(pid.Pid); err != nil {
		return terminate(err)
	}
	if err := json.NewEncoder(parent).Encode(&initConfig{
		Config: c.config,
		Args:   args,
	}); err != nil {
		return terminate(err)
	}
	return pid.Pid, nil
}

func (c *linuxContainer) startInitialProcess(cmd *exec.Cmd, args []string) error {
	glog.Info("starting container initial process")
	// create a pipe so that we can syncronize with the namespaced process and
	// pass the state and configuration to the child process
	parent, child, err := newInitPipe()
	if err != nil {
		return err
	}
	defer parent.Close()
	cmd.ExtraFiles = []*os.File{child}
	cmd.SysProcAttr.Cloneflags = c.config.Namespaces.CloneFlags()
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE=standard")
	// if the container is configured to use user namespaces we have to setup the
	// uid:gid mapping on the command.
	if c.config.Namespaces.Contains(configs.NEWUSER) {
		addUidGidMappings(cmd.SysProcAttr, c.config)
		// Default to root user when user namespaces are enabled.
		if cmd.SysProcAttr.Credential == nil {
			cmd.SysProcAttr.Credential = &syscall.Credential{}
		}
	}
	err = cmd.Start()
	child.Close()
	if err != nil {
		return newGenericError(err, SystemError)
	}
	wait := func() (*os.ProcessState, error) {
		ps, err := cmd.Process.Wait()
		if err != nil {
			return nil, newGenericError(err, SystemError)
		}
		// we should kill all processes in cgroup when init is died if we use
		// host PID namespace
		if !c.config.Namespaces.Contains(configs.NEWPID) {
			c.killAllPids()
		}
		return ps, nil
	}
	terminate := func(terr error) error {
		// TODO: log the errors for kill and wait
		cmd.Process.Kill()
		wait()
		return terr
	}
	started, err := system.GetProcessStartTime(cmd.Process.Pid)
	if err != nil {
		return terminate(err)
	}
	// Do this before syncing with child so that no children
	// can escape the cgroup
	if err := c.cgroupManager.Apply(cmd.Process.Pid); err != nil {
		return terminate(err)
	}
	defer func() {
		if err != nil {
			c.cgroupManager.Destroy()
		}
	}()
	var networkState configs.NetworkState
	if err := c.initializeNetworking(cmd.Process.Pid, &networkState); err != nil {
		return terminate(err)
	}
	iconfig := &initConfig{
		Args:         args,
		Config:       c.config,
		NetworkState: &networkState,
	}
	// Start the setup process to setup the init process
	if c.config.Namespaces.Contains(configs.NEWUSER) {
		if err = executeSetupCmd(cmd.Args, cmd.Process.Pid, c.config, iconfig, &networkState); err != nil {
			return terminate(err)
		}
	}
	// send the state to the container's init process then shutdown writes for the parent
	if err := json.NewEncoder(parent).Encode(iconfig); err != nil {
		return terminate(err)
	}
	// shutdown writes for the parent side of the pipe
	if err := syscall.Shutdown(int(parent.Fd()), syscall.SHUT_WR); err != nil {
		return terminate(err)
	}
	// wait for the child process to fully complete and receive an error message
	// if one was encoutered
	var ierr *initError
	if err := json.NewDecoder(parent).Decode(&ierr); err != nil && err != io.EOF {
		return terminate(err)
	}
	if ierr != nil {
		return terminate(ierr)
	}
	c.state.InitPid = cmd.Process.Pid
	c.state.InitStartTime = started
	c.state.NetworkState = networkState
	c.state.CgroupPaths = c.cgroupManager.GetPaths()
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

func (c *linuxContainer) Signal(signal os.Signal) error {
	glog.Infof("sending signal %d to pid %d", signal, c.state.InitPid)
	panic("not implemented")
}

func (c *linuxContainer) OOM() (<-chan struct{}, error) {
	return NotifyOnOOM(c.state)
}

func (c *linuxContainer) updateStateFile() error {
	fnew := filepath.Join(c.root, fmt.Sprintf("%s.new", stateFilename))
	f, err := os.Create(fnew)
	if err != nil {
		return newGenericError(err, SystemError)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(c.state); err != nil {
		f.Close()
		os.Remove(fnew)
		return newGenericError(err, SystemError)
	}
	fname := filepath.Join(c.root, stateFilename)
	if err := os.Rename(fnew, fname); err != nil {
		return newGenericError(err, SystemError)
	}
	return nil
}

// New returns a newly initialized Pipe for communication between processes
func newInitPipe() (parent *os.File, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// Converts IDMap to SysProcIDMap array and adds it to SysProcAttr.
func addUidGidMappings(sys *syscall.SysProcAttr, container *configs.Config) {
	if container.UidMappings != nil {
		sys.UidMappings = make([]syscall.SysProcIDMap, len(container.UidMappings))
		for i, um := range container.UidMappings {
			sys.UidMappings[i].ContainerID = um.ContainerID
			sys.UidMappings[i].HostID = um.HostID
			sys.UidMappings[i].Size = um.Size
		}
	}

	if container.GidMappings != nil {
		sys.GidMappings = make([]syscall.SysProcIDMap, len(container.GidMappings))
		for i, gm := range container.GidMappings {
			sys.GidMappings[i].ContainerID = gm.ContainerID
			sys.GidMappings[i].HostID = gm.HostID
			sys.GidMappings[i].Size = gm.Size
		}
	}
}

// killAllPids iterates over all of the container's processes
// sending a SIGKILL to each process.
func (c *linuxContainer) killAllPids() error {
	glog.Info("killing all processes in container")
	var procs []*os.Process
	c.cgroupManager.Freeze(configs.Frozen)
	pids, err := c.cgroupManager.GetPids()
	if err != nil {
		return err
	}
	for _, pid := range pids {
		// TODO: log err without aborting if we are unable to find
		// a single PID
		if p, err := os.FindProcess(pid); err == nil {
			procs = append(procs, p)
			p.Kill()
		}
	}
	c.cgroupManager.Freeze(configs.Thawed)
	for _, p := range procs {
		p.Wait()
	}
	return err
}

// initializeNetworking creates the container's network stack outside of the namespace and moves
// interfaces into the container's net namespaces if necessary
func (c *linuxContainer) initializeNetworking(nspid int, networkState *configs.NetworkState) error {
	glog.Info("initailzing container's network stack")
	for _, config := range c.config.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.Create(config, nspid, networkState); err != nil {
			return err
		}
	}
	return nil
}

func executeSetupCmd(args []string, ppid int, container *configs.Config, process *initConfig, networkState *configs.NetworkState) error {
	command := exec.Command(args[0], args[1:]...)
	parent, child, err := newInitPipe()
	if err != nil {
		return err
	}
	defer parent.Close()
	command.ExtraFiles = []*os.File{child}
	command.Dir = container.Rootfs
	command.Env = append(command.Env,
		fmt.Sprintf("_LIBCONTAINER_INITPID=%d", ppid),
		fmt.Sprintf("_LIBCONTAINER_INITTYPE=userns_sidecar"))
	err = command.Start()
	child.Close()
	if err != nil {
		return err
	}
	s, err := command.Process.Wait()
	if err != nil {
		return err
	}
	if !s.Success() {
		return &exec.ExitError{s}
	}
	decoder := json.NewDecoder(parent)
	var pid *pid
	if err := decoder.Decode(&pid); err != nil {
		return err
	}
	p, err := os.FindProcess(pid.Pid)
	if err != nil {
		return err
	}
	terminate := func(terr error) error {
		// TODO: log the errors for kill and wait
		p.Kill()
		p.Wait()
		return terr
	}
	// send the state to the container's init process then shutdown writes for the parent
	if err := json.NewEncoder(parent).Encode(process); err != nil {
		return terminate(err)
	}
	// shutdown writes for the parent side of the pipe
	if err := syscall.Shutdown(int(parent.Fd()), syscall.SHUT_WR); err != nil {
		return terminate(err)
	}
	// wait for the child process to fully complete and receive an error message
	// if one was encoutered
	var ierr *initError
	if err := decoder.Decode(&ierr); err != nil && err != io.EOF {
		return terminate(err)
	}
	if ierr != nil {
		return ierr
	}
	s, err = p.Wait()
	if err != nil {
		return err
	}
	if !s.Success() {
		return &exec.ExitError{s}
	}
	return nil
}

func (c *linuxContainer) enterCgroups(pid int) error {
	return cgroups.EnterPid(c.state.CgroupPaths, pid)
}

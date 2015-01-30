// +build linux

package namespaces

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/system"
)

const (
	EXIT_SIGNAL_OFFSET = 128
)

func executeSetupCmd(args []string, ppid int, container *configs.Config, process *processArgs, networkState *network.NetworkState) error {
	command := exec.Command(args[0], args[1:]...)

	parent, child, err := newInitPipe()
	if err != nil {
		return err
	}
	defer parent.Close()
	command.ExtraFiles = []*os.File{child}

	command.Dir = container.RootFs
	command.Env = append(command.Env,
		fmt.Sprintf("_LIBCONTAINER_INITPID=%d", ppid),
		fmt.Sprintf("_LIBCONTAINER_USERNS=1"))

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

// TODO(vishh): This is part of the libcontainer API and it does much more than just namespaces related work.
// Move this to libcontainer package.
// Exec performs setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(args []string, env []string, console string, command *exec.Cmd, container *configs.Config, cgroupManager cgroups.Manager, state *configs.State) (err error) {
	// create a pipe so that we can syncronize with the namespaced process and
	// pass the state and configuration to the child process
	parent, child, err := newInitPipe()
	if err != nil {
		return err
	}
	defer parent.Close()
	command.ExtraFiles = []*os.File{child}

	command.Dir = container.RootFs
	command.SysProcAttr.Cloneflags = uintptr(GetNamespaceFlags(container.Namespaces))

	if container.Namespaces.Contains(configs.NEWUSER) {
		AddUidGidMappings(command.SysProcAttr, container)

		// Default to root user when user namespaces are enabled.
		if command.SysProcAttr.Credential == nil {
			command.SysProcAttr.Credential = &syscall.Credential{}
		}
	}

	if err := command.Start(); err != nil {
		child.Close()
		return err
	}
	child.Close()

	wait := func() (*os.ProcessState, error) {
		ps, err := command.Process.Wait()
		// we should kill all processes in cgroup when init is died if we use
		// host PID namespace
		if !container.Namespaces.Contains(configs.NEWPID) {
			killAllPids(cgroupManager)
		}
		return ps, err
	}

	terminate := func(terr error) error {
		// TODO: log the errors for kill and wait
		command.Process.Kill()
		wait()
		return terr
	}

	started, err := system.GetProcessStartTime(command.Process.Pid)
	if err != nil {
		return terminate(err)
	}

	// Do this before syncing with child so that no children
	// can escape the cgroup
	err = cgroupManager.Apply(command.Process.Pid)
	if err != nil {
		return terminate(err)
	}
	defer func() {
		if err != nil {
			cgroupManager.Destroy()
		}
	}()

	var networkState network.NetworkState
	if err := InitializeNetworking(container, command.Process.Pid, &networkState); err != nil {
		return terminate(err)
	}

	process := processArgs{
		Env:          append(env[0:], container.Env...),
		Args:         args,
		ConsolePath:  console,
		Config:       container,
		NetworkState: &networkState,
	}

	// Start the setup process to setup the init process
	if container.Namespaces.Contains(configs.NEWUSER) {
		if err = executeSetupCmd(command.Args, command.Process.Pid, container, &process, &networkState); err != nil {
			return terminate(err)
		}
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
	if err := json.NewDecoder(parent).Decode(&ierr); err != nil && err != io.EOF {
		return terminate(err)
	}
	if ierr != nil {
		return terminate(ierr)
	}

	state.InitPid = command.Process.Pid
	state.InitStartTime = started
	state.NetworkState = networkState
	state.CgroupPaths = cgroupManager.GetPaths()

	return nil
}

// killAllPids iterates over all of the container's processes
// sending a SIGKILL to each process.
func killAllPids(m cgroups.Manager) error {
	var (
		procs []*os.Process
	)
	m.Freeze(cgroups.Frozen)
	pids, err := m.GetPids()
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
	m.Freeze(cgroups.Thawed)
	for _, p := range procs {
		p.Wait()
	}
	return err
}

// Utility function that gets a host ID for a container ID from user namespace map
// if that ID is present in the map.
func hostIDFromMapping(containerID int, uMap []configs.IDMap) (int, bool) {
	for _, m := range uMap {
		if (containerID >= m.ContainerID) && (containerID <= (m.ContainerID + m.Size - 1)) {
			hostID := m.HostID + (containerID - m.ContainerID)
			return hostID, true
		}
	}
	return -1, false
}

// Gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled.
func GetHostRootGid(container *configs.Config) (int, error) {
	if container.Namespaces.Contains(configs.NEWUSER) {
		if container.GidMappings == nil {
			return -1, fmt.Errorf("User namespaces enabled, but no gid mappings found.")
		}
		hostRootGid, found := hostIDFromMapping(0, container.GidMappings)
		if !found {
			return -1, fmt.Errorf("User namespaces enabled, but no root user mapping found.")
		}
		return hostRootGid, nil
	}

	// Return default root uid 0
	return 0, nil
}

// Gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled.
func GetHostRootUid(container *configs.Config) (int, error) {
	if container.Namespaces.Contains(configs.NEWUSER) {
		if container.UidMappings == nil {
			return -1, fmt.Errorf("User namespaces enabled, but no user mappings found.")
		}
		hostRootUid, found := hostIDFromMapping(0, container.UidMappings)
		if !found {
			return -1, fmt.Errorf("User namespaces enabled, but no root user mapping found.")
		}
		return hostRootUid, nil
	}

	// Return default root uid 0
	return 0, nil
}

// Converts IDMap to SysProcIDMap array and adds it to SysProcAttr.
func AddUidGidMappings(sys *syscall.SysProcAttr, container *configs.Config) {
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

// InitializeNetworking creates the container's network stack outside of the namespace and moves
// interfaces into the container's net namespaces if necessary
func InitializeNetworking(container *configs.Config, nspid int, networkState *network.NetworkState) error {
	for _, config := range container.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.Create((*network.Network)(config), nspid, networkState); err != nil {
			return err
		}
	}
	return nil
}

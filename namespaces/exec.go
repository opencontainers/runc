// +build linux

package namespaces

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/system"
)

// TODO(vishh): This is part of the libcontainer API and it does much more than just namespaces related work.
// Move this to libcontainer package.
// Exec performs setup outside of a namespace so that a container can be
// executed.  Exec is a high level function for working with container namespaces.
func Exec(args []string, env []string, console string, command *exec.Cmd, container *configs.Config, cgroupManager cgroups.Manager, state *configs.State) error {
	var err error

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

	if err := command.Start(); err != nil {
		child.Close()
		return err
	}
	child.Close()

	terminate := func(terr error) error {
		// TODO: log the errors for kill and wait
		command.Process.Kill()
		command.Wait()
		return terr
	}

	encoder := json.NewEncoder(parent)

	if err := encoder.Encode(container); err != nil {
		return terminate(err)
	}

	process := processArgs{
		Env:         append(env[0:], container.Env...),
		Args:        args,
		ConsolePath: console,
	}
	if err := encoder.Encode(process); err != nil {
		return terminate(err)
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
	defer cgroupManager.Destroy()

	var networkState network.NetworkState
	if err := InitializeNetworking(container, command.Process.Pid, &networkState); err != nil {
		return terminate(err)
	}
	// send the state to the container's init process then shutdown writes for the parent
	if err := encoder.Encode(networkState); err != nil {
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

// DefaultCreateCommand will return an exec.Cmd with the Cloneflags set to the proper namespaces
// defined on the container's configuration and use the current binary as the init with the
// args provided
//
// console: the /dev/console to setup inside the container
// init: the program executed inside the namespaces
// root: the path to the container json file and information
// pipe: sync pipe to synchronize the parent and child processes
// args: the arguments to pass to the container to run as the user's program
func DefaultCreateCommand(container *configs.Config, console, dataPath, init string, pipe *os.File, args []string) *exec.Cmd {
	// get our binary name from arg0 so we can always reexec ourself
	env := []string{
		"console=" + console,
		"pipe=3",
		"data_path=" + dataPath,
	}

	command := exec.Command(init, append([]string{"init", "--"}, args...)...)
	// make sure the process is executed inside the context of the rootfs
	command.Dir = container.RootFs
	command.Env = append(os.Environ(), env...)

	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.Cloneflags = uintptr(GetNamespaceFlags(container.Namespaces))

	command.SysProcAttr.Pdeathsig = syscall.SIGKILL
	command.ExtraFiles = []*os.File{pipe}

	return command
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

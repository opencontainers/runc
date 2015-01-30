// +build linux

package namespaces

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"

	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/mount"
	"github.com/docker/libcontainer/system"
	"github.com/docker/libcontainer/utils"
)

type pid struct {
	Pid int `json:"Pid"`
}

// ExecIn reexec's cmd with _LIBCONTAINER_INITPID=PID so that it is able to run the
// setns code in a single threaded environment joining the existing containers' namespaces.
func ExecIn(args []string, env []string, console string, cmd *exec.Cmd, container *configs.Config, state *configs.State) (int, error) {
	var err error

	parent, child, err := newInitPipe()
	if err != nil {
		return -1, err
	}
	defer parent.Close()

	cmd.ExtraFiles = []*os.File{child}
	cmd.Env = append(cmd.Env, fmt.Sprintf("_LIBCONTAINER_INITPID=%d", state.InitPid))

	if err := cmd.Start(); err != nil {
		child.Close()
		return -1, err
	}
	child.Close()

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
		p.Kill()
		p.Wait()
		return -1, terr
	}

	// Enter cgroups.
	if err := EnterCgroups(state, pid.Pid); err != nil {
		return terminate(err)
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

	return pid.Pid, nil
}

// Finalize entering into a container and execute a specified command
func InitIn(pipe *os.File) (err error) {
	defer func() {
		// if we have an error during the initialization of the container's init then send it back to the
		// parent process in the form of an initError.
		if err != nil {
			// ensure that any data sent from the parent is consumed so it doesn't
			// receive ECONNRESET when the child writes to the pipe.
			ioutil.ReadAll(pipe)
			if err := json.NewEncoder(pipe).Encode(initError{
				Message: err.Error(),
			}); err != nil {
				panic(err)
			}
		}
		// ensure that this pipe is always closed
		pipe.Close()
	}()

	decoder := json.NewDecoder(pipe)

	var container *configs.Config
	if err := decoder.Decode(&container); err != nil {
		return err
	}

	var process *processArgs
	if err := decoder.Decode(&process); err != nil {
		return err
	}

	if err := FinalizeSetns(container); err != nil {
		return err
	}

	if err := system.Execv(process.Args[0], process.Args[0:], process.Env); err != nil {
		return err
	}

	panic("unreachable")
}

// Finalize expects that the setns calls have been setup and that is has joined an
// existing namespace
func FinalizeSetns(container *configs.Config) error {
	// clear the current processes env and replace it with the environment defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}

	if err := setupRlimits(container); err != nil {
		return fmt.Errorf("setup rlimits %s", err)
	}

	if err := FinalizeNamespace(container); err != nil {
		return err
	}

	if err := apparmor.ApplyProfile(container.AppArmorProfile); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", container.AppArmorProfile, err)
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	return nil
}

// SetupContainer is run to setup mounts and networking related operations
// for a user namespace enabled process as a user namespace root doesn't
// have permissions to perform these operations.
// The setup process joins all the namespaces of user namespace enabled init
// except the user namespace, so it run as root in the root user namespace
// to perform these operations.
func SetupContainer(process *processArgs) error {
	container := process.Config
	networkState := process.NetworkState
	consolePath := process.ConsolePath

	rootfs, err := utils.ResolveRootfs(container.RootFs)
	if err != nil {
		return err
	}

	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}

	cloneFlags := GetNamespaceFlags(container.Namespaces)

	if (cloneFlags & syscall.CLONE_NEWNET) == 0 {
		if len(container.Networks) != 0 || len(container.Routes) != 0 {
			return fmt.Errorf("unable to apply network parameters without network namespace")
		}
	} else {
		if err := setupNetwork(container, networkState); err != nil {
			return fmt.Errorf("setup networking %s", err)
		}
		if err := setupRoute(container); err != nil {
			return fmt.Errorf("setup route %s", err)
		}
	}

	label.Init()

	hostRootUid, err := GetHostRootUid(container)
	if err != nil {
		return fmt.Errorf("failed to get hostRootUid %s", err)
	}

	hostRootGid, err := GetHostRootGid(container)
	if err != nil {
		return fmt.Errorf("failed to get hostRootGid %s", err)
	}

	// InitializeMountNamespace() can be executed only for a new mount namespace
	if (cloneFlags & syscall.CLONE_NEWNS) == 0 {
		if container.MountConfig != nil {
			return fmt.Errorf("mount config is set without mount namespace")
		}
	} else if err := mount.InitializeMountNamespace(rootfs,
		consolePath,
		container.RestrictSys,
		hostRootUid,
		hostRootGid,
		(*mount.MountConfig)(container.MountConfig)); err != nil {
		return fmt.Errorf("setup mount namespace %s", err)
	}

	return nil
}

func EnterCgroups(state *configs.State, pid int) error {
	return cgroups.EnterPid(state.CgroupPaths, pid)
}

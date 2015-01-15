// +build linux

package namespaces

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"

	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/system"
)

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

	terminate := func(terr error) (int, error) {
		// TODO: log the errors for kill and wait
		cmd.Process.Kill()
		cmd.Wait()
		return -1, terr
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

	// Enter cgroups.
	if err := EnterCgroups(state, cmd.Process.Pid); err != nil {
		return terminate(err)
	}

	if err := json.NewEncoder(parent).Encode(container); err != nil {
		return terminate(err)
	}

	return cmd.Process.Pid, nil
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

func EnterCgroups(state *configs.State, pid int) error {
	return cgroups.EnterPid(state.CgroupPaths, pid)
}

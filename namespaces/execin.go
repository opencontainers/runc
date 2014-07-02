// +build linux

package namespaces

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/system"
)

// Runs the command under 'args' inside an existing container referred to by 'container'. This API currently does not support a Tty based 'term'.
// Returns the exitcode of the command upon success and appropriate error on failure.
func RunIn(container *libcontainer.Config, state *libcontainer.State, args []string, nsinitPath string, term Terminal, startCallback func()) (int, error) {
	containerJson, err := getContainerJson(container)
	if err != nil {
		return -1, err
	}

	var cmd exec.Cmd

	cmd.Path = nsinitPath
	cmd.Args = getNsEnterCommand(nsinitPath, strconv.Itoa(state.InitPid), containerJson, args)

	if err := term.Attach(&cmd); err != nil {
		return -1, err
	}
	defer term.Close()

	if err := cmd.Start(); err != nil {
		return -1, err
	}
	if startCallback != nil {
		startCallback()
	}

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// ExecIn uses an existing pid and joins the pid's namespaces with the new command.
func ExecIn(container *libcontainer.Config, state *libcontainer.State, args []string) error {
	containerJson, err := getContainerJson(container)
	if err != nil {
		return err
	}
	// Enter the namespace and then finish setup
	finalArgs := getNsEnterCommand(os.Args[0], strconv.Itoa(state.InitPid), containerJson, args)

	if err := system.Execv(finalArgs[0], finalArgs[0:], os.Environ()); err != nil {
		return err
	}
	panic("unreachable")
}

func getContainerJson(container *libcontainer.Config) (string, error) {
	// TODO(vmarmol): If this gets too long, send it over a pipe to the child.
	// Marshall the container into JSON since it won't be available in the namespace.
	containerJson, err := json.Marshal(container)
	if err != nil {
		return "", err
	}
	return string(containerJson), nil
}

func getNsEnterCommand(nsinitPath, initPid, containerJson string, args []string) []string {
	return append([]string{
		nsinitPath,
		"nsenter",
		"--nspid", initPid,
		"--containerjson", containerJson,
		"--",
	}, args...)
}

// Run a command in a container after entering the namespace.
func NsEnter(container *libcontainer.Config, args []string) error {
	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := LoadContainerEnvironment(container); err != nil {
		return err
	}
	if err := FinalizeNamespace(container); err != nil {
		return err
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	if err := system.Execv(args[0], args[0:], container.Env); err != nil {
		return err
	}
	panic("unreachable")
}

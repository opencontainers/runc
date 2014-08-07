// +build linux

package namespaces

import (
	"io"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/syncpipe"
	"github.com/docker/libcontainer/system"
)

func ExecIn(container *libcontainer.Config, state *libcontainer.State, userArgs []string, initPath string,
	stdin io.Reader, stdout, stderr io.Writer, console string, startCallback func(*exec.Cmd)) (int, error) {

	args := []string{"--nspid", strconv.Itoa(state.InitPid)}

	if console != "" {
		args = append(args, "--console", console)
	}

	args = append(args, "nsenter", "--")
	args = append(args, userArgs...)

	cmd := exec.Command(initPath, args...)

	pipe, err := syncpipe.NewSyncPipe()
	if err != nil {
		return -1, err
	}
	defer pipe.Close()

	// Note: these are only used in non-tty mode
	// if there is a tty for the container it will be opened within the namespace and the
	// fds will be duped to stdin, stdiout, and stderr
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	cmd.ExtraFiles = []*os.File{pipe.Child()}

	if err := cmd.Start(); err != nil {
		return -1, err
	}
	pipe.CloseChild()

	if err := pipe.SendToChild(container); err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return -1, err
	}

	if startCallback != nil {
		startCallback(cmd)
	}

	if err := cmd.Wait(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return -1, err
		}
	}

	return cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus(), nil
}

// Finalize expects that the setns calls have been setup and that is has joined an
// existing namespace
func FinalizeSetns(container *libcontainer.Config, args []string) error {
	// clear the current processes env and replace it with the environment defined on the container
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

package libcontainer

import (
	"errors"
	"os"
	"os/exec"
	"sync"

	"github.com/opencontainers/runc/libcontainer/system"
)

func newRestoredProcess(cmd *exec.Cmd, fds []string) (*restoredProcess, error) {
	var err error
	pid := cmd.Process.Pid
	stat, err := system.Stat(pid)
	if err != nil {
		return nil, err
	}
	return &restoredProcess{
		cmd:              cmd,
		processStartTime: stat.StartTime,
		fds:              fds,
	}, nil
}

type restoredProcess struct {
	cmd              *exec.Cmd
	processStartTime uint64
	fds              []string
}

func (p *restoredProcess) start() error {
	return errors.New("restored process cannot be started")
}

func (p *restoredProcess) pid() int {
	return p.cmd.Process.Pid
}

func (p *restoredProcess) terminate() error {
	err := p.cmd.Process.Kill()
	if _, werr := p.wait(); err == nil {
		err = werr
	}
	return err
}

func (p *restoredProcess) wait() (*os.ProcessState, error) {
	// TODO: how do we wait on the actual process?
	// maybe use --exec-cmd in criu
	err := p.cmd.Wait()
	if err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			return nil, err
		}
	}
	st := p.cmd.ProcessState
	return st, nil
}

func (p *restoredProcess) withHandle(fn func(handle uintptr)) error {
	if p.cmd.Process == nil {
		return os.ErrNoHandle
	}
	return p.cmd.Process.WithHandle(fn)
}

func (p *restoredProcess) startTime() (uint64, error) {
	return p.processStartTime, nil
}

func (p *restoredProcess) signal(s os.Signal) error {
	return p.cmd.Process.Signal(s)
}

func (p *restoredProcess) externalDescriptors() []string {
	return p.fds
}

func (p *restoredProcess) setExternalDescriptors(newFds []string) {
	p.fds = newFds
}

func (p *restoredProcess) forwardChildLogs() chan error {
	return nil
}

// nonChildProcess represents a process where the calling process is not
// the parent process. This process is created when Load loads a container
// from a persisted state.
type nonChildProcess struct {
	processPid       int
	processStartTime uint64
	fds              []string

	procOnce sync.Once
	proc     *os.Process
}

// osProcess returns an [os.Process] for the container init.
//
// Unlike the other parentProcess implementations, we have no [exec.Cmd] to
// take it from, so it has to be looked up by pid, which is inherently racy
// (the pid could have been reused before we get here). Once found, though, it
// is kept and reused: on Linux it holds a pidfd, so every subsequent operation
// on it is guaranteed to affect this very process, and never a recycled pid.
// If the pid is already gone by then, the result is a process permanently
// marked as done, which is exactly what every caller wants to hear.
//
// The lookup is lazy because most things one can do with a loaded container
// (state, list, ps, ...) never need it, and it costs an fd.
func (p *nonChildProcess) osProcess() *os.Process {
	p.procOnce.Do(func() {
		// On Unix, FindProcess never fails.
		p.proc, _ = os.FindProcess(p.processPid)
	})
	return p.proc
}

func (p *nonChildProcess) start() error {
	return errors.New("restored process cannot be started")
}

func (p *nonChildProcess) pid() int {
	return p.processPid
}

func (p *nonChildProcess) terminate() error {
	return errors.New("restored process cannot be terminated")
}

func (p *nonChildProcess) wait() (*os.ProcessState, error) {
	return nil, errors.New("restored process cannot be waited on")
}

func (p *nonChildProcess) withHandle(fn func(handle uintptr)) error {
	return p.osProcess().WithHandle(fn)
}

func (p *nonChildProcess) startTime() (uint64, error) {
	return p.processStartTime, nil
}

func (p *nonChildProcess) signal(s os.Signal) error {
	return p.osProcess().Signal(s)
}

func (p *nonChildProcess) externalDescriptors() []string {
	return p.fds
}

func (p *nonChildProcess) setExternalDescriptors(newFds []string) {
	p.fds = newFds
}

func (p *nonChildProcess) forwardChildLogs() chan error {
	return nil
}

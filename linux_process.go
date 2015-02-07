// +build linux

package libcontainer

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
	"github.com/golang/glog"
)

type parentProcess interface {
	// pid returns the pid for the running process.
	pid() int

	// start starts the process execution.
	start() error

	// send a SIGKILL to the process and wait for the exit.
	terminate() error

	// wait waits on the process returning the process state.
	wait() (*os.ProcessState, error)

	// startTime return's the process start time.
	startTime() (string, error)
}

type setnsProcess struct {
	cmd           *exec.Cmd
	parentPipe    *os.File
	childPipe     *os.File
	forkedProcess *os.Process
	cgroupPaths   map[string]string
	config        *initConfig
}

func (p *setnsProcess) startTime() (string, error) {
	return system.GetProcessStartTime(p.pid())
}

func (p *setnsProcess) start() (err error) {
	defer p.parentPipe.Close()
	if p.forkedProcess, err = p.execSetns(); err != nil {
		return err
	}
	if len(p.cgroupPaths) > 0 {
		if err := cgroups.EnterPid(p.cgroupPaths, p.forkedProcess.Pid); err != nil {
			return err
		}
	}
	if err := json.NewEncoder(p.parentPipe).Encode(p.config); err != nil {
		return err
	}
	return nil
}

// execSetns runs the process that executes C code to perform the setns calls
// because setns support requires the C process to fork off a child and perform the setns
// before the go runtime boots, we wait on the process to die and receive the child's pid
// over the provided pipe.
func (p *setnsProcess) execSetns() (*os.Process, error) {
	err := p.cmd.Start()
	p.childPipe.Close()
	if err != nil {
		return nil, err
	}
	status, err := p.cmd.Process.Wait()
	if err != nil {
		return nil, err
	}
	if !status.Success() {
		return nil, &exec.ExitError{status}
	}
	var pid *pid
	if err := json.NewDecoder(p.parentPipe).Decode(&pid); err != nil {
		return nil, err
	}
	return os.FindProcess(pid.Pid)
}

// terminate sends a SIGKILL to the forked process for the setns routine then waits to
// avoid the process becomming a zombie.
func (p *setnsProcess) terminate() error {
	if p.forkedProcess == nil {
		return nil
	}
	err := p.forkedProcess.Kill()
	if _, werr := p.wait(); err == nil {
		err = werr
	}
	return err
}

func (p *setnsProcess) wait() (*os.ProcessState, error) {
	return p.forkedProcess.Wait()
}

func (p *setnsProcess) pid() int {
	return p.forkedProcess.Pid
}

type initProcess struct {
	cmd        *exec.Cmd
	parentPipe *os.File
	childPipe  *os.File
	config     *initConfig
	manager    cgroups.Manager
}

func (p *initProcess) pid() int {
	return p.cmd.Process.Pid
}

func (p *initProcess) start() error {
	defer p.parentPipe.Close()
	err := p.cmd.Start()
	p.childPipe.Close()
	if err != nil {
		return err
	}
	// Do this before syncing with child so that no children
	// can escape the cgroup
	if err := p.manager.Apply(p.pid()); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			// TODO: should not be the responsibility to call here
			p.manager.Destroy()
		}
	}()
	if err := p.createNetworkInterfaces(); err != nil {
		return err
	}
	// Start the setup process to setup the init process
	if p.cmd.SysProcAttr.Cloneflags&syscall.CLONE_NEWUSER != 0 {
		parent, err := p.newUsernsSetupProcess()
		if err != nil {
			return err
		}
		if err := parent.start(); err != nil {
			if err := parent.terminate(); err != nil {
				glog.Warning(err)
			}
			return err
		}
		if _, err := parent.wait(); err != nil {
			return err
		}
	}
	if err := p.sendConfig(); err != nil {
		return err
	}
	// wait for the child process to fully complete and receive an error message
	// if one was encoutered
	var ierr *initError
	if err := json.NewDecoder(p.parentPipe).Decode(&ierr); err != nil && err != io.EOF {
		return err
	}
	if ierr != nil {
		return ierr
	}
	return nil
}

func (p *initProcess) wait() (*os.ProcessState, error) {
	state, err := p.cmd.Process.Wait()
	if err != nil {
		return nil, err
	}
	// we should kill all processes in cgroup when init is died if we use host PID namespace
	if p.cmd.SysProcAttr.Cloneflags&syscall.CLONE_NEWPID == 0 {
		// TODO: this will not work for the success path because libcontainer
		// does not wait on the process.  This needs to be moved to destroy or add a Wait()
		// method back onto the container.
		var procs []*os.Process
		p.manager.Freeze(configs.Frozen)
		pids, err := p.manager.GetPids()
		if err != nil {
			return nil, err
		}
		for _, pid := range pids {
			// TODO: log err without aborting if we are unable to find
			// a single PID
			if p, err := os.FindProcess(pid); err == nil {
				procs = append(procs, p)
				p.Kill()
			}
		}
		p.manager.Freeze(configs.Thawed)
		for _, p := range procs {
			p.Wait()
		}
	}
	return state, nil
}

func (p *initProcess) terminate() error {
	if p.cmd.Process == nil {
		return nil
	}
	err := p.cmd.Process.Kill()
	if _, werr := p.wait(); err == nil {
		err = werr
	}
	return err
}

func (p *initProcess) startTime() (string, error) {
	return system.GetProcessStartTime(p.pid())
}

func (p *initProcess) sendConfig() error {
	// send the state to the container's init process then shutdown writes for the parent
	if err := json.NewEncoder(p.parentPipe).Encode(p.config); err != nil {
		return err
	}
	// shutdown writes for the parent side of the pipe
	return syscall.Shutdown(int(p.parentPipe.Fd()), syscall.SHUT_WR)
}

func (p *initProcess) createNetworkInterfaces() error {
	for _, config := range p.config.Config.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.Create(config, p.pid()); err != nil {
			return err
		}
	}
	return nil
}

func (p *initProcess) newUsernsSetupProcess() (parentProcess, error) {
	parentPipe, childPipe, err := newPipe()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(p.cmd.Args[0], p.cmd.Args[1:]...)
	cmd.ExtraFiles = []*os.File{childPipe}
	cmd.Dir = p.cmd.Dir
	cmd.Env = append(cmd.Env,
		fmt.Sprintf("_LIBCONTAINER_INITPID=%d", p.pid()),
		fmt.Sprintf("_LIBCONTAINER_INITTYPE=userns_setup"),
	)
	return &setnsProcess{
		cmd:        cmd,
		childPipe:  childPipe,
		parentPipe: parentPipe,
		config:     p.config,
	}, nil
}

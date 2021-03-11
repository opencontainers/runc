package util

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runtime-spec/specs-go"
)

type CtAct uint8

const (
	CT_ACT_CREATE CtAct = iota + 1
	CT_ACT_RUN
	CT_ACT_RESTORE
)

type Runner struct {
	Init            bool
	EnableSubreaper bool
	ShouldDestroy   bool
	Detach          bool
	ListenFDs       []*os.File
	PreserveFDs     int
	PidFile         string
	ConsoleSocket   string
	Container       libcontainer.Container
	Action          CtAct
	NotifySocket    *notifySocket
	CriuOpts        *libcontainer.CriuOpts
	LogLevel        string
}

func (r *Runner) Run(config *specs.Process) (int, error) {
	var err error
	defer func() {
		if err != nil {
			r.destroy()
		}
	}()
	if err = r.checkTerminal(config); err != nil {
		return -1, err
	}
	process, err := newProcess(*config, r.Init, r.LogLevel)
	if err != nil {
		return -1, err
	}
	if len(r.ListenFDs) > 0 {
		process.Env = append(process.Env, "LISTEN_FDS="+strconv.Itoa(len(r.ListenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.ListenFDs...)
	}
	baseFd := 3 + len(process.ExtraFiles)
	for i := baseFd; i < baseFd+r.PreserveFDs; i++ {
		_, err = os.Stat("/proc/self/fd/" + strconv.Itoa(i))
		if err != nil {
			return -1, errors.Wrapf(err, "please check that preserved-fd %d (of %d) is present", i-baseFd, r.PreserveFDs)
		}
		process.ExtraFiles = append(process.ExtraFiles, os.NewFile(uintptr(i), "PreserveFD:"+strconv.Itoa(i)))
	}
	rootuid, err := r.Container.Config().HostRootUID()
	if err != nil {
		return -1, err
	}
	rootgid, err := r.Container.Config().HostRootGID()
	if err != nil {
		return -1, err
	}
	var (
		detach = r.Detach || (r.Action == CT_ACT_CREATE)
	)
	// Setting up IO is a two stage process. We need to modify process to deal
	// with detaching containers, and then we get a tty after the Container has
	// started.
	handler := newSignalHandler(r.EnableSubreaper, r.NotifySocket)
	tty, err := setupIO(process, rootuid, rootgid, config.Terminal, detach, r.ConsoleSocket)
	if err != nil {
		return -1, err
	}
	defer tty.Close()

	switch r.Action {
	case CT_ACT_CREATE:
		err = r.Container.Start(process)
	case CT_ACT_RESTORE:
		err = r.Container.Restore(process, r.CriuOpts)
	case CT_ACT_RUN:
		err = r.Container.Run(process)
	default:
		panic("Unknown Action")
	}
	if err != nil {
		return -1, err
	}
	if err = tty.waitConsole(); err != nil {
		r.terminate(process)
		return -1, err
	}
	if err = tty.ClosePostStart(); err != nil {
		r.terminate(process)
		return -1, err
	}
	if r.PidFile != "" {
		if err = createPidFile(r.PidFile, process); err != nil {
			r.terminate(process)
			return -1, err
		}
	}
	status, err := handler.forward(process, tty, detach)
	if err != nil {
		r.terminate(process)
	}
	if detach {
		return 0, nil
	}
	if err == nil {
		r.destroy()
	}
	return status, err
}

func (r *Runner) destroy() {
	if r.ShouldDestroy {
		Destroy(r.Container)
	}
}

func (r *Runner) terminate(p *libcontainer.Process) {
	_ = p.Signal(unix.SIGKILL)
	_, _ = p.Wait()
}

func (r *Runner) checkTerminal(config *specs.Process) error {
	detach := r.Detach || (r.Action == CT_ACT_CREATE)
	// Check command-line for sanity.
	if detach && config.Terminal && r.ConsoleSocket == "" {
		return errors.New("cannot allocate tty if runc will Detach without setting console socket")
	}
	if (!detach || !config.Terminal) && r.ConsoleSocket != "" {
		return errors.New("cannot use console socket if runc will not Detach or allocate tty")
	}
	return nil
}

// newProcess returns a new libcontainer Process with the arguments from the
// spec and stdio from the current process.
func newProcess(p specs.Process, init bool, logLevel string) (*libcontainer.Process, error) {
	lp := &libcontainer.Process{
		Args: p.Args,
		Env:  p.Env,
		// TODO: fix libcontainer's API to better support uid/gid in a typesafe way.
		User:            fmt.Sprintf("%d:%d", p.User.UID, p.User.GID),
		Cwd:             p.Cwd,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
		Init:            init,
		LogLevel:        logLevel,
	}

	if p.ConsoleSize != nil {
		lp.ConsoleWidth = uint16(p.ConsoleSize.Width)
		lp.ConsoleHeight = uint16(p.ConsoleSize.Height)
	}

	if p.Capabilities != nil {
		lp.Capabilities = &configs.Capabilities{}
		lp.Capabilities.Bounding = p.Capabilities.Bounding
		lp.Capabilities.Effective = p.Capabilities.Effective
		lp.Capabilities.Inheritable = p.Capabilities.Inheritable
		lp.Capabilities.Permitted = p.Capabilities.Permitted
		lp.Capabilities.Ambient = p.Capabilities.Ambient
	}
	for _, gid := range p.User.AdditionalGids {
		lp.AdditionalGroups = append(lp.AdditionalGroups, strconv.FormatUint(uint64(gid), 10))
	}
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}
	return lp, nil
}

func createLibContainerRlimit(rlimit specs.POSIXRlimit) (configs.Rlimit, error) {
	rl, err := StrToRlimit(rlimit.Type)
	if err != nil {
		return configs.Rlimit{}, err
	}
	return configs.Rlimit{
		Type: rl,
		Hard: rlimit.Hard,
		Soft: rlimit.Soft,
	}, nil
}

// setupIO modifies the given process config according to the options.
func setupIO(process *libcontainer.Process, rootuid, rootgid int, createTTY, detach bool, sockpath string) (*tty, error) {
	if createTTY {
		process.Stdin = nil
		process.Stdout = nil
		process.Stderr = nil
		t := &tty{}
		if !detach {
			if err := t.initHostConsole(); err != nil {
				return nil, err
			}
			parent, child, err := utils.NewSockPair("console")
			if err != nil {
				return nil, err
			}
			process.ConsoleSocket = child
			t.postStart = append(t.postStart, parent, child)
			t.consoleC = make(chan error, 1)
			go func() {
				t.consoleC <- t.recvtty(process, parent)
			}()
		} else {
			// the caller of runc will handle receiving the console master
			conn, err := net.Dial("unix", sockpath)
			if err != nil {
				return nil, err
			}
			uc, ok := conn.(*net.UnixConn)
			if !ok {
				return nil, errors.New("casting to UnixConn failed")
			}
			t.postStart = append(t.postStart, uc)
			socket, err := uc.File()
			if err != nil {
				return nil, err
			}
			t.postStart = append(t.postStart, socket)
			process.ConsoleSocket = socket
		}
		return t, nil
	}
	// when runc will Detach the caller provides the stdio to runc via runc's 0,1,2
	// and the Container's process inherits runc's stdio.
	if detach {
		if err := inheritStdio(process); err != nil {
			return nil, err
		}
		return &tty{}, nil
	}
	return setupProcessPipes(process, rootuid, rootgid)
}

// createPidFile creates a file with the processes pid inside it atomically
// it creates a temp file with the paths filename + '.' infront of it
// then renames the file
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, "."+filepath.Base(path))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(strconv.Itoa(pid))
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

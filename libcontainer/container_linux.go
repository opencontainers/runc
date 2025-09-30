package libcontainer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/exeseal"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/utils"
)

const stdioFdCount = 3

// Container is a libcontainer container object.
type Container struct {
	id                   string
	stateDir             string
	config               *configs.Config
	cgroupManager        cgroups.Manager
	intelRdtManager      *intelrdt.Manager
	initProcess          parentProcess
	initProcessStartTime uint64
	m                    sync.Mutex
	criuVersion          int
	state                containerState
	created              time.Time
	fifo                 *os.File
}

// State represents a running container's state
type State struct {
	BaseState

	// Platform specific fields below.

	// Specified if the container was started under the rootless mode.
	// Set to true if BaseState.Config.RootlessEUID && BaseState.Config.RootlessCgroups
	Rootless bool `json:"rootless,omitempty"`

	// Paths to all the container's cgroups, as returned by (*cgroups.Manager).GetPaths
	//
	// For cgroup v1, a key is cgroup subsystem name, and the value is the path
	// to the cgroup for this subsystem.
	//
	// For cgroup v2 unified hierarchy, a key is "", and the value is the unified path.
	CgroupPaths map[string]string `json:"cgroup_paths,omitempty"`

	// NamespacePaths are filepaths to the container's namespaces. Key is the namespace type
	// with the value as the path.
	NamespacePaths map[configs.NamespaceType]string `json:"namespace_paths"`

	// Container's standard descriptors (std{in,out,err}), needed for checkpoint and restore.
	ExternalDescriptors []string `json:"external_descriptors,omitempty"`

	// Intel RDT "resource control" filesystem path.
	IntelRdtPath string `json:"intel_rdt_path,omitempty"`
}

// ID returns the container's unique ID
func (c *Container) ID() string {
	return c.id
}

// Config returns the container's configuration
func (c *Container) Config() configs.Config {
	return *c.config
}

// Status returns the current status of the container.
func (c *Container) Status() (Status, error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.currentStatus()
}

// State returns the current container's state information.
func (c *Container) State() (*State, error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.currentState(), nil
}

// OCIState returns the current container's state information.
func (c *Container) OCIState() (*specs.State, error) {
	c.m.Lock()
	defer c.m.Unlock()
	return c.currentOCIState()
}

// ignoreCgroupError filters out cgroup-related errors that can be ignored,
// because the container is stopped and its cgroup is gone.
func (c *Container) ignoreCgroupError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) && !c.hasInit() && !c.cgroupManager.Exists() {
		return nil
	}
	return err
}

// Processes returns the PIDs inside this container. The PIDs are in the
// namespace of the calling process.
//
// Some of the returned PIDs may no longer refer to processes in the container,
// unless the container state is PAUSED in which case every PID in the slice is
// valid.
func (c *Container) Processes() ([]int, error) {
	pids, err := c.cgroupManager.GetAllPids()
	if err = c.ignoreCgroupError(err); err != nil {
		return nil, fmt.Errorf("unable to get all container pids: %w", err)
	}
	return pids, nil
}

// Stats returns statistics for the container.
func (c *Container) Stats() (*Stats, error) {
	var (
		err   error
		stats = &Stats{}
	)
	if stats.CgroupStats, err = c.cgroupManager.GetStats(); err != nil {
		return stats, fmt.Errorf("unable to get container cgroup stats: %w", err)
	}
	if c.intelRdtManager != nil {
		if stats.IntelRdtStats, err = c.intelRdtManager.GetStats(); err != nil {
			return stats, fmt.Errorf("unable to get container Intel RDT stats: %w", err)
		}
	}
	for _, iface := range c.config.Networks {
		switch iface.Type {
		case "veth":
			istats, err := getNetworkInterfaceStats(iface.HostInterfaceName)
			if err != nil {
				return stats, fmt.Errorf("unable to get network stats for interface %q: %w", iface.HostInterfaceName, err)
			}
			stats.Interfaces = append(stats.Interfaces, istats)
		}
	}
	return stats, nil
}

// Set resources of container as configured. Can be used to change resources
// when the container is running.
func (c *Container) Set(config configs.Config) error {
	c.m.Lock()
	defer c.m.Unlock()
	status, err := c.currentStatus()
	if err != nil {
		return err
	}
	if status == Stopped {
		return ErrNotRunning
	}
	if err := c.cgroupManager.Set(config.Cgroups.Resources); err != nil {
		// Set configs back
		if err2 := c.cgroupManager.Set(c.config.Cgroups.Resources); err2 != nil {
			logrus.Warnf("Setting back cgroup configs failed due to error: %v, your state.json and actual configs might be inconsistent.", err2)
		}
		return err
	}
	if c.intelRdtManager != nil {
		if err := c.intelRdtManager.Set(&config); err != nil {
			// Set configs back
			if err2 := c.cgroupManager.Set(c.config.Cgroups.Resources); err2 != nil {
				logrus.Warnf("Setting back cgroup configs failed due to error: %v, your state.json and actual configs might be inconsistent.", err2)
			}
			if err2 := c.intelRdtManager.Set(c.config); err2 != nil {
				logrus.Warnf("Setting back intelrdt configs failed due to error: %v, your state.json and actual configs might be inconsistent.", err2)
			}
			return err
		}
	}
	// After config setting succeed, update config and states
	c.config = &config
	_, err = c.updateState(nil)
	return err
}

// Start starts a process inside the container. Returns error if process fails
// to start. You can track process lifecycle with passed Process structure.
func (c *Container) Start(process *Process) error {
	c.m.Lock()
	defer c.m.Unlock()
	return c.start(process)
}

// Run immediately starts the process inside the container. Returns an error if
// the process fails to start. It does not block waiting for the exec fifo
// after start returns but opens the fifo after start returns.
func (c *Container) Run(process *Process) error {
	c.m.Lock()
	defer c.m.Unlock()
	if err := c.start(process); err != nil {
		return err
	}
	if process.Init {
		return c.exec()
	}
	return nil
}

// Exec signals the container to exec the users process at the end of the init.
func (c *Container) Exec() error {
	c.m.Lock()
	defer c.m.Unlock()
	return c.exec()
}

func (c *Container) exec() error {
	path := filepath.Join(c.stateDir, execFifoFilename)
	pid := c.initProcess.pid()
	blockingFifoOpenCh := awaitFifoOpen(path)
	for {
		select {
		case result := <-blockingFifoOpenCh:
			return handleFifoResult(result)

		case <-time.After(time.Millisecond * 100):
			stat, err := system.Stat(pid)
			if err != nil || stat.State == system.Zombie {
				// could be because process started, ran, and completed between our 100ms timeout and our system.Stat() check.
				// see if the fifo exists and has data (with a non-blocking open, which will succeed if the writing process is complete).
				if err := handleFifoResult(fifoOpen(path, false)); err != nil {
					return errors.New("container process is already dead")
				}
				return nil
			}
		}
	}
}

func readFromExecFifo(execFifo io.Reader) error {
	data, err := io.ReadAll(execFifo)
	if err != nil {
		return err
	}
	if len(data) <= 0 {
		return errors.New("cannot start an already running container")
	}
	return nil
}

func awaitFifoOpen(path string) <-chan openResult {
	fifoOpened := make(chan openResult)
	go func() {
		result := fifoOpen(path, true)
		fifoOpened <- result
	}()
	return fifoOpened
}

func fifoOpen(path string, block bool) openResult {
	flags := os.O_RDONLY
	if !block {
		flags |= unix.O_NONBLOCK
	}
	f, err := os.OpenFile(path, flags, 0)
	if err != nil {
		return openResult{err: fmt.Errorf("exec fifo: %w", err)}
	}
	return openResult{file: f}
}

func handleFifoResult(result openResult) error {
	if result.err != nil {
		return result.err
	}
	f := result.file
	defer f.Close()
	if err := readFromExecFifo(f); err != nil {
		return err
	}
	err := os.Remove(f.Name())
	if err == nil || os.IsNotExist(err) {
		return nil
	}
	return err
}

type openResult struct {
	file *os.File
	err  error
}

func (c *Container) start(process *Process) (retErr error) {
	if c.config.Cgroups.Resources.SkipDevices {
		return errors.New("can't start container with SkipDevices set")
	}

	if c.config.RootlessEUID && len(process.AdditionalGroups) > 0 {
		// We cannot set any additional groups in a rootless container
		// and thus we bail if the user asked us to do so.
		return errors.New("cannot set any additional groups in a rootless container")
	}

	if process.Init {
		if c.initProcessStartTime != 0 {
			return errors.New("container already has init process")
		}
		if err := c.createExecFifo(); err != nil {
			return err
		}
		defer func() {
			if retErr != nil {
				c.deleteExecFifo()
			}
		}()
	}

	parent, err := c.newParentProcess(process)
	if err != nil {
		return fmt.Errorf("unable to create new parent process: %w", err)
	}
	// We do not need the cloned binaries once the process is spawned.
	defer process.closeClonedExes()

	logsDone := parent.forwardChildLogs()

	// Before starting "runc init", mark all non-stdio open files as O_CLOEXEC
	// to make sure we don't leak any files into "runc init". Any files to be
	// passed to "runc init" through ExtraFiles will get dup2'd by the Go
	// runtime and thus their O_CLOEXEC flag will be cleared. This is some
	// additional protection against attacks like CVE-2024-21626, by making
	// sure we never leak files to "runc init" we didn't intend to.
	if err := utils.CloseExecFrom(3); err != nil {
		return fmt.Errorf("unable to mark non-stdio fds as cloexec: %w", err)
	}
	if err := parent.start(); err != nil {
		return fmt.Errorf("unable to start container process: %w", err)
	}

	if logsDone != nil {
		defer func() {
			// Wait for log forwarder to finish. This depends on
			// runc init closing the _LIBCONTAINER_LOGPIPE log fd.
			err := <-logsDone
			if err != nil && retErr == nil {
				retErr = fmt.Errorf("unable to forward init logs: %w", err)
			}
		}()
	}

	if process.Init {
		c.fifo.Close()
		if c.config.HasHook(configs.Poststart) {
			s, err := c.currentOCIState()
			if err != nil {
				return err
			}

			if err := c.config.Hooks.Run(configs.Poststart, s); err != nil {
				if err := ignoreTerminateErrors(parent.terminate()); err != nil {
					logrus.Warn(fmt.Errorf("error running poststart hook: %w", err))
				}
				return err
			}
		}
	}
	return nil
}

// Signal sends a specified signal to container's init.
//
// When s is SIGKILL:
// 1. If the container does not have its own PID namespace, all the
// container's processes are killed. In this scenario, the libcontainer
// user may be required to implement a proper child reaper.
// 2. Otherwise, we just send the SIGKILL signal to the init process,
// but we don't wait for the init process to disappear. If you want to
// wait, please use c.EnsureKilled instead.
func (c *Container) Signal(s os.Signal) error {
	c.m.Lock()
	defer c.m.Unlock()

	// When a container has its own PID namespace, inside it the init PID
	// is 1, and thus it is handled specially by the kernel. In particular,
	// killing init with SIGKILL from an ancestor namespace will also kill
	// all other processes in that PID namespace (see pid_namespaces(7)).
	//
	// OTOH, if PID namespace is shared, we should kill all pids to avoid
	// leftover processes. Handle this special case here.
	if s == unix.SIGKILL && !c.config.Namespaces.IsPrivate(configs.NEWPID) {
		if err := signalAllProcesses(c.cgroupManager, unix.SIGKILL); err != nil {
			if c.config.RootlessCgroups { // may not have an access to cgroup
				logrus.WithError(err).Warn("failed to kill all processes, possibly due to lack of cgroup (Hint: enable cgroup v2 delegation)")
				// Some processes may leak when cgroup is not delegated
				// https://github.com/opencontainers/runc/pull/4395#pullrequestreview-2291179652
				return c.signal(s)
			}
			// For not rootless container, if there is no init process and no cgroup,
			// it means that the container is not running.
			if errors.Is(err, ErrCgroupNotExist) && !c.hasInit() {
				err = ErrNotRunning
			}
			return fmt.Errorf("unable to kill all processes: %w", err)
		}
		return nil
	}

	return c.signal(s)
}

func (c *Container) signal(s os.Signal) error {
	// To avoid a PID reuse attack, don't kill non-running container.
	if !c.hasInit() {
		return ErrNotRunning
	}
	if err := c.initProcess.signal(s); err != nil {
		return fmt.Errorf("unable to signal init: %w", err)
	}
	if s == unix.SIGKILL {
		// For cgroup v1, killing a process in a frozen cgroup
		// does nothing until it's thawed. Only thaw the cgroup
		// for SIGKILL.
		if paused, _ := c.isPaused(); paused {
			_ = c.cgroupManager.Freeze(cgroups.Thawed)
		}
	}
	return nil
}

func (c *Container) killViaPidfd() error {
	c.m.Lock()
	defer c.m.Unlock()

	// To avoid a PID reuse attack, don't kill non-running container.
	if !c.hasInit() {
		return ErrNotRunning
	}

	pidfd, err := unix.PidfdOpen(c.initProcess.pid(), 0)
	if err != nil {
		return err
	}
	defer unix.Close(pidfd)

	epollfd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return err
	}
	defer unix.Close(epollfd)

	event := unix.EpollEvent{
		Events: unix.EPOLLIN,
		Fd:     int32(pidfd),
	}
	if err := unix.EpollCtl(epollfd, unix.EPOLL_CTL_ADD, pidfd, &event); err != nil {
		return err
	}

	if err := unix.PidfdSendSignal(pidfd, unix.SIGKILL, nil, 0); err != nil {
		return err
	}

	events := make([]unix.EpollEvent, 1)
	// Set the timeout to 10s, the same as in kill below.
	n, err := linux.EpollWait(epollfd, events, 10000)
	if err != nil {
		return err
	}
	if n > 0 {
		for i := range n {
			event := events[i]
			if event.Fd == int32(pidfd) {
				return nil
			}
		}
	}
	return errors.New("container init still running")
}

func (c *Container) kill() error {
	_ = c.Signal(unix.SIGKILL)

	// For containers running in a low load machine, we only need to wait about 1ms.
	time.Sleep(time.Millisecond)
	if err := c.Signal(unix.Signal(0)); err != nil {
		return nil
	}

	// For some containers in a heavy load machine, we need to wait more time.
	logrus.Debugln("We need more time to wait the init process exit.")
	for i := 0; i < 100; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := c.Signal(unix.Signal(0)); err != nil {
			return nil
		}
	}
	return errors.New("container init still running")
}

// EnsureKilled kills the container and waits for the kernel to finish killing it.
func (c *Container) EnsureKilled() error {
	// When a container doesn't have a private pidns, we have to kill all processes
	// in the cgroup, it's more simpler to use `cgroup.kill` or `unix.Kill`.
	if c.config.Namespaces.IsPrivate(configs.NEWPID) {
		var err error
		if err = c.killViaPidfd(); err == nil {
			return nil
		}

		logrus.Debugf("pidfd & epoll failed, falling back to unix.Signal: %v", err)
	}
	return c.kill()
}

func (c *Container) createExecFifo() (retErr error) {
	rootuid, err := c.config.HostRootUID()
	if err != nil {
		return err
	}
	rootgid, err := c.config.HostRootGID()
	if err != nil {
		return err
	}

	fifoName := filepath.Join(c.stateDir, execFifoFilename)
	if err := unix.Mkfifo(fifoName, 0o622); err != nil {
		return &os.PathError{Op: "mkfifo", Path: fifoName, Err: err}
	}
	defer func() {
		if retErr != nil {
			os.Remove(fifoName)
		}
	}()
	// Ensure permission bits (can be different because of umask).
	if err := os.Chmod(fifoName, 0o622); err != nil {
		return err
	}
	return os.Chown(fifoName, rootuid, rootgid)
}

func (c *Container) deleteExecFifo() {
	fifoName := filepath.Join(c.stateDir, execFifoFilename)
	os.Remove(fifoName)
}

// includeExecFifo opens the container's execfifo as a pathfd, so that the
// container cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with _LIBCONTAINER_FIFOFD set to its fd number.
func (c *Container) includeExecFifo(cmd *exec.Cmd) error {
	fifoName := filepath.Join(c.stateDir, execFifoFilename)
	fifo, err := os.OpenFile(fifoName, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	c.fifo = fifo

	cmd.ExtraFiles = append(cmd.ExtraFiles, fifo)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_FIFOFD="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1))
	return nil
}

func (c *Container) newParentProcess(p *Process) (parentProcess, error) {
	comm, err := newProcessComm()
	if err != nil {
		return nil, err
	}

	// Make sure we use a new safe copy of /proc/self/exe binary each time, this
	// is called to make sure that if a container manages to overwrite the file,
	// it cannot affect other containers on the system. For runc, this code will
	// only ever be called once, but libcontainer users might call this more than
	// once.
	p.closeClonedExes()
	var (
		exePath string
		safeExe *os.File
	)
	if exeseal.IsSelfExeCloned() {
		// /proc/self/exe is already a cloned binary -- no need to do anything
		logrus.Debug("skipping binary cloning -- /proc/self/exe is already cloned!")
		// We don't need to use /proc/thread-self here because the exe mm of a
		// thread-group is guaranteed to be the same for all threads by
		// definition. This lets us avoid having to do runtime.LockOSThread.
		exePath = "/proc/self/exe"
	} else {
		var err error
		safeExe, err = exeseal.CloneSelfExe(c.stateDir)
		if err != nil {
			return nil, fmt.Errorf("unable to create safe /proc/self/exe clone for runc init: %w", err)
		}
		exePath = "/proc/self/fd/" + strconv.Itoa(int(safeExe.Fd()))
		p.clonedExes = append(p.clonedExes, safeExe)
		logrus.Debug("runc exeseal: using /proc/self/exe clone") // used for tests
	}

	cmd := exec.Command(exePath, "init")
	cmd.Args[0] = os.Args[0]
	cmd.Stdin = p.Stdin
	cmd.Stdout = p.Stdout
	cmd.Stderr = p.Stderr
	cmd.Dir = c.config.Rootfs
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &unix.SysProcAttr{}
	}
	cmd.Env = append(cmd.Env, "GOMAXPROCS="+os.Getenv("GOMAXPROCS"))
	cmd.ExtraFiles = append(cmd.ExtraFiles, p.ExtraFiles...)
	if p.ConsoleSocket != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, p.ConsoleSocket)
		cmd.Env = append(cmd.Env,
			"_LIBCONTAINER_CONSOLE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		)
	}

	cmd.ExtraFiles = append(cmd.ExtraFiles, comm.initSockChild)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_INITPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
	)
	cmd.ExtraFiles = append(cmd.ExtraFiles, comm.syncSockChild.File())
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_SYNCPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
	)

	cmd.ExtraFiles = append(cmd.ExtraFiles, comm.logPipeChild)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_LOGPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1))
	if p.LogLevel != "" {
		cmd.Env = append(cmd.Env, "_LIBCONTAINER_LOGLEVEL="+p.LogLevel)
	}

	if p.PidfdSocket != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, p.PidfdSocket)
		cmd.Env = append(cmd.Env,
			"_LIBCONTAINER_PIDFD_SOCK="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		)
	}

	// TODO: After https://go-review.googlesource.com/c/go/+/515799 included
	// in go versions supported by us, we can remove this logic.
	if safeExe != nil {
		// Due to a Go stdlib bug, we need to add safeExe to the set of
		// ExtraFiles otherwise it is possible for the stdlib to clobber the fd
		// during forkAndExecInChild1 and replace it with some other file that
		// might be malicious. This is less than ideal (because the descriptor
		// will be non-O_CLOEXEC) however we have protections in "runc init" to
		// stop us from leaking extra file descriptors.
		//
		// See <https://github.com/golang/go/issues/61751>.
		cmd.ExtraFiles = append(cmd.ExtraFiles, safeExe)

		// There is a race situation when we are opening a file, if there is a
		// small fd was closed at that time, maybe it will be reused by safeExe.
		// Because of Go stdlib fds shuffling bug, if the fd of safeExe is too
		// small, go stdlib will dup3 it to another fd, or dup3 a other fd to this
		// fd, then it will cause the fd type cmd.Path refers to a random path,
		// and it can lead to an error "permission denied" when starting the process.
		// Please see #4294.
		// So we should not use the original fd of safeExe, but use the fd after
		// shuffled by Go stdlib. Because Go stdlib will guarantee this fd refers to
		// the correct file.
		cmd.Path = "/proc/self/fd/" + strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1)
	}

	// NOTE: when running a container with no PID namespace and the parent
	//       process spawning the container is PID1 the pdeathsig is being
	//       delivered to the container's init process by the kernel for some
	//       reason even with the parent still running.
	if c.config.ParentDeathSignal > 0 {
		cmd.SysProcAttr.Pdeathsig = unix.Signal(c.config.ParentDeathSignal)
	}

	if p.Init {
		// We only set up fifoFd if we're not doing a `runc exec`. The historic
		// reason for this is that previously we would pass a dirfd that allowed
		// for container rootfs escape (and not doing it in `runc exec` avoided
		// that problem), but we no longer do that. However, there's no need to do
		// this for `runc exec` so we just keep it this way to be safe.
		if err := c.includeExecFifo(cmd); err != nil {
			return nil, fmt.Errorf("unable to setup exec fifo: %w", err)
		}
		return c.newInitProcess(p, cmd, comm)
	}
	return c.newSetnsProcess(p, cmd, comm)
}

func (c *Container) newInitProcess(p *Process, cmd *exec.Cmd, comm *processComm) (*initProcess, error) {
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE="+string(initStandard))
	nsMaps := make(map[configs.NamespaceType]string)
	for _, ns := range c.config.Namespaces {
		if ns.Path != "" {
			nsMaps[ns.Type] = ns.Path
		}
	}
	data, err := c.bootstrapData(c.config.Namespaces.CloneFlags(), nsMaps)
	if err != nil {
		return nil, err
	}

	init := &initProcess{
		containerProcess: containerProcess{
			cmd:           cmd,
			comm:          comm,
			manager:       c.cgroupManager,
			config:        c.newInitConfig(p),
			process:       p,
			bootstrapData: data,
			container:     c,
		},
		intelRdtManager: c.intelRdtManager,
	}
	c.initProcess = init
	return init, nil
}

func (c *Container) newSetnsProcess(p *Process, cmd *exec.Cmd, comm *processComm) (*setnsProcess, error) {
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE="+string(initSetns))
	state := c.currentState()
	// for setns process, we don't have to set cloneflags as the process namespaces
	// will only be set via setns syscall
	data, err := c.bootstrapData(0, state.NamespacePaths)
	if err != nil {
		return nil, err
	}
	proc := &setnsProcess{
		containerProcess: containerProcess{
			cmd:           cmd,
			comm:          comm,
			manager:       c.cgroupManager,
			config:        c.newInitConfig(p),
			process:       p,
			bootstrapData: data,
			container:     c,
		},
		rootlessCgroups: c.config.RootlessCgroups,
		intelRdtPath:    state.IntelRdtPath,
		initProcessPid:  state.InitProcessPid,
	}
	return proc, nil
}

func (c *Container) newInitConfig(process *Process) *initConfig {
	// Set initial properties. For those properties that exist
	// both in the container config and the process, use the ones
	// from the container config first, and override them later.
	cfg := &initConfig{
		Config:           c.config,
		Args:             process.Args,
		Env:              process.Env,
		UID:              process.UID,
		GID:              process.GID,
		AdditionalGroups: process.AdditionalGroups,
		Cwd:              process.Cwd,
		Capabilities:     c.config.Capabilities,
		PassedFilesCount: len(process.ExtraFiles),
		ContainerID:      c.ID(),
		NoNewPrivileges:  c.config.NoNewPrivileges,
		AppArmorProfile:  c.config.AppArmorProfile,
		ProcessLabel:     c.config.ProcessLabel,
		Rlimits:          c.config.Rlimits,
		IOPriority:       c.config.IOPriority,
		Scheduler:        c.config.Scheduler,
		CPUAffinity:      c.config.ExecCPUAffinity,
		CreateConsole:    process.ConsoleSocket != nil,
		ConsoleWidth:     process.ConsoleWidth,
		ConsoleHeight:    process.ConsoleHeight,
	}

	// Overwrite config properties with ones from process.

	if process.Capabilities != nil {
		cfg.Capabilities = process.Capabilities
	}
	if process.NoNewPrivileges != nil {
		cfg.NoNewPrivileges = *process.NoNewPrivileges
	}
	if process.AppArmorProfile != "" {
		cfg.AppArmorProfile = process.AppArmorProfile
	}
	if process.Label != "" {
		cfg.ProcessLabel = process.Label
	}
	if len(process.Rlimits) > 0 {
		cfg.Rlimits = process.Rlimits
	}
	if process.IOPriority != nil {
		cfg.IOPriority = process.IOPriority
	}
	if process.Scheduler != nil {
		cfg.Scheduler = process.Scheduler
	}
	if process.CPUAffinity != nil {
		cfg.CPUAffinity = process.CPUAffinity
	}

	// Set misc properties.

	if cgroups.IsCgroup2UnifiedMode() {
		cfg.Cgroup2Path = c.cgroupManager.Path("")
	}

	return cfg
}

// Destroy destroys the container, if its in a valid state.
//
// Any event registrations are removed before the container is destroyed.
// No error is returned if the container is already destroyed.
//
// Running containers must first be stopped using Signal.
// Paused containers must first be resumed using Resume.
func (c *Container) Destroy() error {
	c.m.Lock()
	defer c.m.Unlock()
	if err := c.state.destroy(); err != nil {
		return fmt.Errorf("unable to destroy container: %w", err)
	}
	return nil
}

// Pause pauses the container, if its state is RUNNING or CREATED, changing
// its state to PAUSED. If the state is already PAUSED, does nothing.
func (c *Container) Pause() error {
	c.m.Lock()
	defer c.m.Unlock()
	status, err := c.currentStatus()
	if err != nil {
		return err
	}
	switch status {
	case Running, Created:
		if err := c.cgroupManager.Freeze(cgroups.Frozen); err != nil {
			return err
		}
		return c.state.transition(&pausedState{
			c: c,
		})
	}
	return ErrNotRunning
}

// Resume resumes the execution of any user processes in the
// container before setting the container state to RUNNING.
// This is only performed if the current state is PAUSED.
// If the Container state is RUNNING, does nothing.
func (c *Container) Resume() error {
	c.m.Lock()
	defer c.m.Unlock()
	status, err := c.currentStatus()
	if err != nil {
		return err
	}
	if status != Paused {
		return ErrNotPaused
	}
	if err := c.cgroupManager.Freeze(cgroups.Thawed); err != nil {
		return err
	}
	return c.state.transition(&runningState{
		c: c,
	})
}

// NotifyOOM returns a read-only channel signaling when the container receives
// an OOM notification.
func (c *Container) NotifyOOM() (<-chan struct{}, error) {
	// XXX(cyphar): This requires cgroups.
	if c.config.RootlessCgroups {
		logrus.Warn("getting OOM notifications may fail if you don't have the full access to cgroups")
	}
	path := c.cgroupManager.Path("memory")
	if cgroups.IsCgroup2UnifiedMode() {
		return notifyOnOOMV2(path)
	}
	return notifyOnOOM(path)
}

// NotifyMemoryPressure returns a read-only channel signaling when the
// container reaches a given pressure level.
func (c *Container) NotifyMemoryPressure(level PressureLevel) (<-chan struct{}, error) {
	// XXX(cyphar): This requires cgroups.
	if c.config.RootlessCgroups {
		logrus.Warn("getting memory pressure notifications may fail if you don't have the full access to cgroups")
	}
	return notifyMemoryPressure(c.cgroupManager.Path("memory"), level)
}

func (c *Container) updateState(process parentProcess) (*State, error) {
	if process != nil {
		c.initProcess = process
	}
	state := c.currentState()
	if err := c.saveState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func (c *Container) saveState(s *State) (retErr error) {
	tmpFile, err := os.CreateTemp(c.stateDir, "state-")
	if err != nil {
		return err
	}

	defer func() {
		if retErr != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
		}
	}()

	err = utils.WriteJSON(tmpFile, s)
	if err != nil {
		return err
	}
	err = tmpFile.Close()
	if err != nil {
		return err
	}

	stateFilePath := filepath.Join(c.stateDir, stateFilename)
	return os.Rename(tmpFile.Name(), stateFilePath)
}

func (c *Container) currentStatus() (Status, error) {
	if err := c.refreshState(); err != nil {
		return -1, err
	}
	return c.state.status(), nil
}

// refreshState needs to be called to verify that the current state on the
// container is what is true.  Because consumers of libcontainer can use it
// out of process we need to verify the container's status based on runtime
// information and not rely on our in process info.
func (c *Container) refreshState() error {
	paused, err := c.isPaused()
	if err != nil {
		return err
	}
	if paused {
		return c.state.transition(&pausedState{c: c})
	}
	if !c.hasInit() {
		return c.state.transition(&stoppedState{c: c})
	}
	// The presence of exec fifo helps to distinguish between
	// the created and the running states.
	if _, err := os.Stat(filepath.Join(c.stateDir, execFifoFilename)); err == nil {
		return c.state.transition(&createdState{c: c})
	}
	return c.state.transition(&runningState{c: c})
}

// hasInit tells whether the container init process exists.
func (c *Container) hasInit() bool {
	if c.initProcess == nil {
		return false
	}
	pid := c.initProcess.pid()
	stat, err := system.Stat(pid)
	if err != nil {
		return false
	}
	if stat.StartTime != c.initProcessStartTime || stat.State == system.Zombie || stat.State == system.Dead {
		return false
	}
	return true
}

func (c *Container) isPaused() (bool, error) {
	state, err := c.cgroupManager.GetFreezerState()
	if err != nil {
		return false, err
	}
	return state == cgroups.Frozen, nil
}

func (c *Container) currentState() *State {
	var (
		startTime           uint64
		externalDescriptors []string
		pid                 = -1
	)
	if c.initProcess != nil {
		pid = c.initProcess.pid()
		startTime, _ = c.initProcess.startTime()
		externalDescriptors = c.initProcess.externalDescriptors()
	}

	intelRdtPath := ""
	if c.intelRdtManager != nil {
		intelRdtPath = c.intelRdtManager.GetPath()
	}
	state := &State{
		BaseState: BaseState{
			ID:                   c.ID(),
			Config:               *c.config,
			InitProcessPid:       pid,
			InitProcessStartTime: startTime,
			Created:              c.created,
		},
		Rootless:            c.config.RootlessEUID && c.config.RootlessCgroups,
		CgroupPaths:         c.cgroupManager.GetPaths(),
		IntelRdtPath:        intelRdtPath,
		NamespacePaths:      make(map[configs.NamespaceType]string),
		ExternalDescriptors: externalDescriptors,
	}
	if pid > 0 {
		for _, ns := range c.config.Namespaces {
			state.NamespacePaths[ns.Type] = ns.GetPath(pid)
		}
		for _, nsType := range configs.NamespaceTypes() {
			if !configs.IsNamespaceSupported(nsType) {
				continue
			}
			if _, ok := state.NamespacePaths[nsType]; !ok {
				ns := configs.Namespace{Type: nsType}
				state.NamespacePaths[ns.Type] = ns.GetPath(pid)
			}
		}
	}
	return state
}

func (c *Container) currentOCIState() (*specs.State, error) {
	bundle, annotations := utils.Annotations(c.config.Labels)
	state := &specs.State{
		Version:     specs.Version,
		ID:          c.ID(),
		Bundle:      bundle,
		Annotations: annotations,
	}
	status, err := c.currentStatus()
	if err != nil {
		return nil, err
	}
	state.Status = specs.ContainerState(status.String())
	if status != Stopped {
		if c.initProcess != nil {
			state.Pid = c.initProcess.pid()
		}
	}
	return state, nil
}

// orderNamespacePaths sorts namespace paths into a list of paths that we
// can setns in order.
func (c *Container) orderNamespacePaths(namespaces map[configs.NamespaceType]string) ([]string, error) {
	paths := []string{}
	for _, ns := range configs.NamespaceTypes() {

		// Remove namespaces that we don't need to join.
		if !c.config.Namespaces.Contains(ns) {
			continue
		}

		if p, ok := namespaces[ns]; ok && p != "" {
			// check if the requested namespace is supported
			if !configs.IsNamespaceSupported(ns) {
				return nil, fmt.Errorf("namespace %s is not supported", ns)
			}
			// only set to join this namespace if it exists
			if _, err := os.Lstat(p); err != nil {
				return nil, fmt.Errorf("namespace path: %w", err)
			}
			// do not allow namespace path with comma as we use it to separate
			// the namespace paths
			if strings.ContainsRune(p, ',') {
				return nil, fmt.Errorf("invalid namespace path %s", p)
			}
			paths = append(paths, fmt.Sprintf("%s:%s", configs.NsName(ns), p))
		}

	}

	return paths, nil
}

func encodeIDMapping(idMap []configs.IDMap) ([]byte, error) {
	data := bytes.NewBuffer(nil)
	for _, im := range idMap {
		line := fmt.Sprintf("%d %d %d\n", im.ContainerID, im.HostID, im.Size)
		if _, err := data.WriteString(line); err != nil {
			return nil, err
		}
	}
	return data.Bytes(), nil
}

// netlinkError is an error wrapper type for use by custom netlink message
// types. Panics with errors are wrapped in netlinkError so that the recover
// in bootstrapData can distinguish intentional panics.
type netlinkError struct{ error }

// bootstrapData encodes the necessary data in netlink binary format
// as a io.Reader.
// Consumer can write the data to a bootstrap program
// such as one that uses nsenter package to bootstrap the container's
// init process correctly, i.e. with correct namespaces, uid/gid
// mapping etc.
func (c *Container) bootstrapData(cloneFlags uintptr, nsMaps map[configs.NamespaceType]string) (_ io.Reader, Err error) {
	// create the netlink message
	r := nl.NewNetlinkRequest(int(InitMsg), 0)

	// Our custom messages cannot bubble up an error using returns, instead
	// they will panic with the specific error type, netlinkError. In that
	// case, recover from the panic and return that as an error.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(netlinkError); ok {
				Err = e.error
			} else {
				panic(r)
			}
		}
	}()

	// write cloneFlags
	r.AddData(&Int32msg{
		Type:  CloneFlagsAttr,
		Value: uint32(cloneFlags),
	})

	// write custom namespace paths
	if len(nsMaps) > 0 {
		nsPaths, err := c.orderNamespacePaths(nsMaps)
		if err != nil {
			return nil, err
		}
		r.AddData(&Bytemsg{
			Type:  NsPathsAttr,
			Value: []byte(strings.Join(nsPaths, ",")),
		})
	}

	// write namespace paths only when we are not joining an existing user ns
	_, joinExistingUser := nsMaps[configs.NEWUSER]
	if !joinExistingUser {
		// write uid mappings
		if len(c.config.UIDMappings) > 0 {
			if c.config.RootlessEUID {
				// We resolve the paths for new{u,g}idmap from
				// the context of runc to avoid doing a path
				// lookup in the nsexec context.
				if path, err := exec.LookPath("newuidmap"); err == nil {
					r.AddData(&Bytemsg{
						Type:  UidmapPathAttr,
						Value: []byte(path),
					})
				}
			}
			b, err := encodeIDMapping(c.config.UIDMappings)
			if err != nil {
				return nil, err
			}
			r.AddData(&Bytemsg{
				Type:  UidmapAttr,
				Value: b,
			})
		}

		// write gid mappings
		if len(c.config.GIDMappings) > 0 {
			b, err := encodeIDMapping(c.config.GIDMappings)
			if err != nil {
				return nil, err
			}
			r.AddData(&Bytemsg{
				Type:  GidmapAttr,
				Value: b,
			})
			if c.config.RootlessEUID {
				if path, err := exec.LookPath("newgidmap"); err == nil {
					r.AddData(&Bytemsg{
						Type:  GidmapPathAttr,
						Value: []byte(path),
					})
				}
			}
			if requiresRootOrMappingTool(c.config) {
				r.AddData(&Boolmsg{
					Type:  SetgroupAttr,
					Value: true,
				})
			}
		}
	}

	if c.config.OomScoreAdj != nil {
		// write oom_score_adj
		r.AddData(&Bytemsg{
			Type:  OomScoreAdjAttr,
			Value: []byte(strconv.Itoa(*c.config.OomScoreAdj)),
		})
	}

	// write rootless
	r.AddData(&Boolmsg{
		Type:  RootlessEUIDAttr,
		Value: c.config.RootlessEUID,
	})

	// write boottime and monotonic time ns offsets only when we are not joining an existing time ns
	_, joinExistingTime := nsMaps[configs.NEWTIME]
	if !joinExistingTime && c.config.TimeOffsets != nil {
		var offsetSpec bytes.Buffer
		for clock, offset := range c.config.TimeOffsets {
			fmt.Fprintf(&offsetSpec, "%s %d %d\n", clock, offset.Secs, offset.Nanosecs)
		}
		r.AddData(&Bytemsg{
			Type:  TimeOffsetsAttr,
			Value: offsetSpec.Bytes(),
		})
	}

	return bytes.NewReader(r.Serialize()), nil
}

// ignoreTerminateErrors returns nil if the given err matches an error known
// to indicate that the terminate occurred successfully or err was nil, otherwise
// err is returned unaltered.
func ignoreTerminateErrors(err error) error {
	if err == nil {
		return nil
	}
	// terminate() might return an error from either Kill or Wait.
	// The (*Cmd).Wait documentation says: "If the command fails to run
	// or doesn't complete successfully, the error is of type *ExitError".
	// Filter out such errors (like "exit status 1" or "signal: killed").
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil
	}
	if errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	s := err.Error()
	if strings.Contains(s, "Wait was already called") {
		return nil
	}
	return err
}

func requiresRootOrMappingTool(c *configs.Config) bool {
	gidMap := []configs.IDMap{
		{ContainerID: 0, HostID: int64(os.Getegid()), Size: 1},
	}
	return !reflect.DeepEqual(c.GIDMappings, gidMap)
}

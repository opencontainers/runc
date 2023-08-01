package libcontainer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moby/sys/mountinfo"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/execabs"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/opencontainers/runc/libcontainer/utils"
)

const stdioFdCount = 3

// Container is a libcontainer container object.
type Container struct {
	id                   string
	root                 string
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
	safeExeFile          *os.File
}

// State represents a running container's state
type State struct {
	BaseState

	// Platform specific fields below here

	// Specified if the container was started under the rootless mode.
	// Set to true if BaseState.Config.RootlessEUID && BaseState.Config.RootlessCgroups
	Rootless bool `json:"rootless"`

	// Paths to all the container's cgroups, as returned by (*cgroups.Manager).GetPaths
	//
	// For cgroup v1, a key is cgroup subsystem name, and the value is the path
	// to the cgroup for this subsystem.
	//
	// For cgroup v2 unified hierarchy, a key is "", and the value is the unified path.
	CgroupPaths map[string]string `json:"cgroup_paths"`

	// NamespacePaths are filepaths to the container's namespaces. Key is the namespace type
	// with the value as the path.
	NamespacePaths map[configs.NamespaceType]string `json:"namespace_paths"`

	// Container's standard descriptors (std{in,out,err}), needed for checkpoint and restore
	ExternalDescriptors []string `json:"external_descriptors,omitempty"`

	// Intel RDT "resource control" filesystem path
	IntelRdtPath string `json:"intel_rdt_path"`
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
	return c.currentState()
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
	if errors.Is(err, os.ErrNotExist) && c.runType() == Stopped && !c.cgroupManager.Exists() {
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
	if c.config.Cgroups.Resources.SkipDevices {
		return errors.New("can't start container with SkipDevices set")
	}
	if process.Init {
		if err := c.createExecFifo(); err != nil {
			return err
		}
	}
	if err := c.start(process); err != nil {
		if process.Init {
			c.deleteExecFifo()
		}
		return err
	}
	return nil
}

// Run immediately starts the process inside the container. Returns an error if
// the process fails to start. It does not block waiting for the exec fifo
// after start returns but opens the fifo after start returns.
func (c *Container) Run(process *Process) error {
	if err := c.Start(process); err != nil {
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
	path := filepath.Join(c.root, execFifoFilename)
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
	return os.Remove(f.Name())
}

type openResult struct {
	file *os.File
	err  error
}

func (c *Container) start(process *Process) (retErr error) {
	parent, err := c.newParentProcess(process)
	if err != nil {
		return fmt.Errorf("unable to create new parent process: %w", err)
	}
	// This is no longer needed after the process has been spawned. We also
	// want to make sure that (especially in the case of O_TMPFILE descriptors)
	// that we use a new copy for each execution, because an attacker
	// overwriting our copy would be just as bad as overwiting the host runc
	// binary if we re-use the copy.
	defer c.clearSafeExe()

	logsDone := parent.forwardChildLogs()
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

	if err := parent.start(); err != nil {
		return fmt.Errorf("unable to start container process: %w", err)
	}

	if process.Init {
		c.fifo.Close()
		if c.config.Hooks != nil {
			s, err := c.currentOCIState()
			if err != nil {
				return err
			}

			if err := c.config.Hooks[configs.Poststart].RunHooks(s); err != nil {
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
// When s is SIGKILL and the container does not have its own PID namespace, all
// the container's processes are killed. In this scenario, the libcontainer
// user may be required to implement a proper child reaper.
func (c *Container) Signal(s os.Signal) error {
	c.m.Lock()
	defer c.m.Unlock()
	status, err := c.currentStatus()
	if err != nil {
		return err
	}
	// To avoid a PID reuse attack, don't kill non-running container.
	switch status {
	case Running, Created, Paused:
	default:
		return ErrNotRunning
	}

	// When a container has its own PID namespace, inside it the init PID
	// is 1, and thus it is handled specially by the kernel. In particular,
	// killing init with SIGKILL from an ancestor namespace will also kill
	// all other processes in that PID namespace (see pid_namespaces(7)).
	//
	// OTOH, if PID namespace is shared, we should kill all pids to avoid
	// leftover processes.
	if s == unix.SIGKILL && !c.config.Namespaces.IsPrivate(configs.NEWPID) {
		err = signalAllProcesses(c.cgroupManager, unix.SIGKILL)
	} else {
		err = c.initProcess.signal(s)
	}
	if err != nil {
		return fmt.Errorf("unable to signal init: %w", err)
	}
	if status == Paused && s == unix.SIGKILL {
		// For cgroup v1, killing a process in a frozen cgroup
		// does nothing until it's thawed. Only thaw the cgroup
		// for SIGKILL.
		_ = c.cgroupManager.Freeze(configs.Thawed)
	}
	return nil
}

func (c *Container) createExecFifo() error {
	rootuid, err := c.Config().HostRootUID()
	if err != nil {
		return err
	}
	rootgid, err := c.Config().HostRootGID()
	if err != nil {
		return err
	}

	fifoName := filepath.Join(c.root, execFifoFilename)
	if _, err := os.Stat(fifoName); err == nil {
		return fmt.Errorf("exec fifo %s already exists", fifoName)
	}
	oldMask := unix.Umask(0o000)
	if err := unix.Mkfifo(fifoName, 0o622); err != nil {
		unix.Umask(oldMask)
		return err
	}
	unix.Umask(oldMask)
	return os.Chown(fifoName, rootuid, rootgid)
}

func (c *Container) deleteExecFifo() {
	fifoName := filepath.Join(c.root, execFifoFilename)
	os.Remove(fifoName)
}

// includeExecFifo opens the container's execfifo as a pathfd, so that the
// container cannot access the statedir (and the FIFO itself remains
// un-opened). It then adds the FifoFd to the given exec.Cmd as an inherited
// fd, with _LIBCONTAINER_FIFOFD set to its fd number.
func (c *Container) includeExecFifo(cmd *exec.Cmd) error {
	fifoName := filepath.Join(c.root, execFifoFilename)
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

func (c *Container) clearSafeExe() {
	if c.safeExeFile != nil {
		_ = c.safeExeFile.Close()
		c.safeExeFile = nil
	}
}

// makeSafeExe makes a copy of /proc/self/exe that is safe to use when
// executing inside a container. On modern kernels, this is a locked executable
// memfd that contains a copy of /proc/self/exe. The returned string is a
// /proc/self/fd/... path that can be used directly with "os/exec".Command().
// For more details on why this is necessary, see CVE-2019-5736.
func (c *Container) makeSafeExe() (path string, Err error) {
	if c.safeExeFile == nil {
		var err error
		var sealFn func(**os.File) error

		// Close safeExeFile if we fail to make it properly.
		defer func() {
			if c.safeExeFile != nil && Err != nil {
				c.clearSafeExe()
			}
		}()

		// First, try an executable memfd (supported since Linux 3.17).
		c.safeExeFile, err = system.ExecutableMemfd("runc_cloned:/proc/self/exe", unix.MFD_ALLOW_SEALING|unix.MFD_CLOEXEC)
		if err != nil {
			logrus.Debugf("memfd cloned binary failed, falling back to O_TMPFILE: %v", err)
		} else {
			sealFn = func(f **os.File) error {
				if err := (*f).Chmod(0o511); err != nil {
					return fmt.Errorf("chmod memfd: %w", err)
				}
				// Try to set the newer memfd sealing flags, but we ignore
				// errors because they are not needed and we want to continue
				// to work on older kernels.
				fd := (*f).Fd()
				// F_SEAL_FUTURE_WRITE -- Linux 5.1
				_, _ = unix.FcntlInt(fd, unix.F_ADD_SEALS, unix.F_SEAL_FUTURE_WRITE)
				// F_SEAL_EXEC -- Linux 6.3
				const F_SEAL_EXEC = 0x2000 //nolint:revive // this matches the unix.* name
				_, _ = unix.FcntlInt(fd, unix.F_ADD_SEALS, F_SEAL_EXEC)
				// Apply all original memfd seals.
				_, err := unix.FcntlInt(fd, unix.F_ADD_SEALS, unix.F_SEAL_SEAL|unix.F_SEAL_SHRINK|unix.F_SEAL_GROW|unix.F_SEAL_WRITE)
				return os.NewSyscallError("fcntl(F_ADD_SEALS)", err)
			}
		}

		// Try to fallback to O_TMPFILE (supported since Linux 3.11).
		if c.safeExeFile == nil {
			var stat unix.Stat_t
			c.safeExeFile, err = os.OpenFile(c.root, unix.O_TMPFILE|unix.O_RDWR|unix.O_EXCL|unix.O_CLOEXEC, 0o700)
			if err != nil {
				logrus.Debugf("O_TMPFILE cloned binary failed, falling back to mktemp(): %v", err)
			} else if err := unix.Fstat(int(c.safeExeFile.Fd()), &stat); err != nil || stat.Nlink != 0 {
				logrus.Debugf("O_TMPFILE cloned binary has non-zero nlink, falling back to mktemp(): %v", err)
				c.clearSafeExe()
			}

			// Finally, fallback to a classic temporary file we unlink.
			if c.safeExeFile == nil {
				c.safeExeFile, err = os.CreateTemp(c.root, "runc.")
				if err != nil {
					return "", fmt.Errorf("could not clone binary: %w", err)
				}
				// Unlink the file and verify it was unlinked.
				if err := os.Remove(c.safeExeFile.Name()); err != nil {
					return "", fmt.Errorf("unlinking classic tmpfile: %w", err)
				}
				if err := unix.Fstat(int(c.safeExeFile.Fd()), &stat); err != nil {
					return "", fmt.Errorf("classic tmpfile fstat: %w", err)
				} else if stat.Nlink != 0 {
					return "", fmt.Errorf("classic tmpfile %s has non-zero nlink after unlink", c.safeExeFile.Name())
				}
			}
			sealFn = func(f **os.File) error {
				if err := (*f).Chmod(0o511); err != nil {
					return fmt.Errorf("chmod tmpfile: %w", err)
				}
				// When sealing an O_TMPFILE-style descriptor we need to
				// re-open the path as O_PATH to clear the existing write
				// handle we have.
				opath, err := os.OpenFile(fmt.Sprintf("/proc/self/fd/%d", (*f).Fd()), unix.O_PATH|unix.O_CLOEXEC, 0)
				if err != nil {
					return fmt.Errorf("reopen tmpfile: %w", err)
				}
				_ = (*f).Close()
				*f = opath
				return nil
			}
		}

		// Copy the contents of /proc/self/exe to the cloned fd.
		srcExeFile, err := os.Open("/proc/self/exe")
		if err != nil {
			return "", fmt.Errorf("cannot open current process exe: %w", err)
		}
		defer srcExeFile.Close()
		stat, err := srcExeFile.Stat()
		if err != nil {
			return "", fmt.Errorf("checking /proc/self/exe size: %w", err)
		}
		exeSize := stat.Size()

		copied, err := system.Copy(c.safeExeFile, srcExeFile)
		if err != nil {
			return "", fmt.Errorf("copy binary: %w", err)
		} else if copied != exeSize {
			return "", fmt.Errorf("copied binary size mismatch: %d != %d", copied, exeSize)
		}

		// Seal the descriptor.
		if err := sealFn(&c.safeExeFile); err != nil {
			return "", fmt.Errorf("could not seal fd: %w", err)
		}
	}
	return fmt.Sprintf("/proc/self/fd/%d", c.safeExeFile.Fd()), nil
}

func (c *Container) newParentProcess(p *Process) (parentProcess, error) {
	parentInitPipe, childInitPipe, err := utils.NewSockPair("init")
	if err != nil {
		return nil, fmt.Errorf("unable to create init pipe: %w", err)
	}
	messageSockPair := filePair{parentInitPipe, childInitPipe}

	parentLogPipe, childLogPipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("unable to create log pipe: %w", err)
	}
	logFilePair := filePair{parentLogPipe, childLogPipe}

	exePath, err := c.makeSafeExe()
	if err != nil {
		return nil, fmt.Errorf("unable to create safe /proc/self/exe clone: %w", err)
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
	cmd.ExtraFiles = append(cmd.ExtraFiles, childInitPipe)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_INITPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		"_LIBCONTAINER_STATEDIR="+c.root,
	)

	cmd.ExtraFiles = append(cmd.ExtraFiles, childLogPipe)
	cmd.Env = append(cmd.Env,
		"_LIBCONTAINER_LOGPIPE="+strconv.Itoa(stdioFdCount+len(cmd.ExtraFiles)-1),
		"_LIBCONTAINER_LOGLEVEL="+p.LogLevel,
	)

	// Due to a Go stdlib bug, we need to add c.safeExeFile to the set of
	// ExtraFiles otherwise it is possible for the stdlib to clobber the fd
	// during forkAndExecInChild1 and replace it with some other file that
	// might be malicious. This is less than ideal (because the descriptor will
	// be non-O_CLOEXEC) however we have protections in "runc init" to stop us
	// from leaking extra file descriptors.
	//
	// See <https://github.com/golang/go/issues/61751>.
	cmd.ExtraFiles = append(cmd.ExtraFiles, c.safeExeFile)

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
		return c.newInitProcess(p, cmd, messageSockPair, logFilePair)
	}
	return c.newSetnsProcess(p, cmd, messageSockPair, logFilePair)
}

// remapMountSources tries to remap any applicable mount sources in the
// container configuration to use open_tree(2)-style mountfds. This allows
// containers to have bind-mounts from source directories the container process
// cannot resolve (#2576) as well as apply MOUNT_ATTR_IDMAP to mounts (which
// requires privileges the container process might not have).
//
// The core idea is that we create a mountfd with the right configuration, and
// then replace the source with a reference to the file descriptor. Ideally
// this would be /proc/self/fd/<the-fd-runc-init-will-see>, but we cannot use
// new mount API file descriptors as bind-mount sources (they run afoul of the
// check_mnt() checks that stop us from doing bind-mounts from an fd in a
// different namespace). So instead, we set the source to \x00<file-descriptor>
// which is an invalid path under Linux.
func (c *Container) remapMountSources(cmd *exec.Cmd) (Err error) {
	nsHandles := new(userns.Handles)
	defer nsHandles.Release()
	for i, m := range c.config.Mounts {
		var mountFile *os.File
		if m.IsBind() {
			flags := uint(unix.OPEN_TREE_CLONE | unix.O_CLOEXEC)
			if m.Flags&unix.MS_REC == unix.MS_REC {
				flags |= unix.AT_RECURSIVE
			}
			mountFd, err := unix.OpenTree(unix.AT_FDCWD, m.Source, flags)
			if err != nil {
				// For non-id-mapped mounts, this functionality is optional. We
				// can just fallback to letting the rootfs code use the
				// original source as a mountpoint.
				if !m.IsIDMapped() {
					logrus.Debugf("remap mount sources: skipping remap of %s due to failure: open_tree(OPEN_TREE_CLONE): %v", m.Source, err)
					continue
				}
				return &os.PathError{Op: "open_tree(OPEN_TREE_CLONE)", Path: m.Source, Err: err}
			}
			mountFile = os.NewFile(uintptr(mountFd), m.Source+" (open_tree)")
			// Only close the file if the remapping failed -- otherwise we keep
			// the file open to be passed to "runc init".
			defer func() {
				if Err != nil {
					_ = mountFile.Close()
				}
			}()
		}

		if m.IsIDMapped() {
			if mountFile == nil {
				return fmt.Errorf("remap mount sources: invalid mount source %s: id-mapping of non-bind-mounts is not supported", m.Source)
			}
			usernsFile, err := nsHandles.Get(userns.Mapping{
				UIDMappings: m.UIDMappings,
				GIDMappings: m.GIDMappings,
			})
			if err != nil {
				return fmt.Errorf("remap mount sources: failed to create userns for %s id-mapping: %w", m.Source, err)
			}
			defer usernsFile.Close()
			if err := unix.MountSetattr(int(mountFile.Fd()), "", unix.AT_EMPTY_PATH, &unix.MountAttr{
				Attr_set:  unix.MOUNT_ATTR_IDMAP,
				Userns_fd: uint64(usernsFile.Fd()),
			}); err != nil {
				return fmt.Errorf("remap mount sources: failed to set IDMAP_SOURCE_ATTR on %s: %w", m.Source, err)
			}
		}

		if mountFile != nil {
			// We need to emulate the propagation behaviour when bind-mounting
			// from a root filesystem with config.RootPropagation applied. This
			// is needed because existing runc users expect bind-mounts to act
			// this way (the "classic" method), regardless of the mount API
			// used to create the bind-mount.
			//
			// NOTE: This explicitly does not handle configurations where there
			// is a bind-mount source of a path from inside the container
			// rootfs. We could do this in rootfs_linux.go but this would
			// require doing criu-style shennanigans to try to figure out how
			// to recreate the state in /proc/self/mountinfo. For now, it's
			// much simpler to implement this based purely on RootPropagation.

			// Same logic as prepareRoot().
			propFlags := unix.MS_SLAVE | unix.MS_REC
			if c.config.RootPropagation != 0 {
				propFlags = c.config.RootPropagation
			}

			// The one thing to consider is whether the RootPropagation is
			// MS_REC and whether the mount source is the same as /. If it is
			// MS_REC then we apply the propagation flags (nix MS_REC)
			// recursively. Otherwise we apply the propagation flags
			// non-recursively if m.Source is part of the / mount. We do
			// nothing if neither is the case.
			var setattrFlags uint
			if propFlags&unix.MS_REC == unix.MS_REC {
				// If the RootPropagation is recursive, then any bind-mount
				// from the host should inherit the propagation setting of /.
				logrus.Debugf("remap mount sources: applying recursive RootPropagation of 0x%x to %q bind-mount propagation", propFlags, m.Source)
				setattrFlags = unix.AT_RECURSIVE
			} else {
				// If the RootPropagation is not recursive, only bind-mounts
				// from paths within the / mount should inherit the propagation
				// setting /.
				info, err := mountinfo.GetMounts(mountinfo.ParentsFilter(m.Source))
				if err != nil {
					return fmt.Errorf("remap mount sources: failed to parse /proc/self/mountinfo: %w", err)
				}
				// If there is only a single entry with a mountpoint path of /,
				// the source was a child of /. Otherwise, do not set the
				// propagation setting of bind-mounts.
				if len(info) == 1 && info[0].Mountpoint == "/" {
					logrus.Debugf("remap mount sources: applying non-recursive RootPropagation of 0x%x to child of / %q bind-mount propagation", propFlags, m.Source)
				} else {
					logrus.Debugf("remap mount sources: using default mount propagation for %q bind-mount", m.Source)
					propFlags = 0
				}
			}
			if propFlags != 0 {
				if err := unix.MountSetattr(int(mountFile.Fd()), "", unix.AT_EMPTY_PATH|setattrFlags, &unix.MountAttr{
					Propagation: uint64(propFlags &^ unix.MS_REC),
				}); err != nil {
					return fmt.Errorf("remap mount sources: failed to set mount propagation of %q bind-mount to 0x%x: %w", m.Source, propFlags, err)
				}
			}

			// We have to leak these file descriptors to "runc init", which
			// will mean that Go will set them as ~O_CLOEXEC during ForkExec,
			// meaning that by default these would be leaked to the container
			// process.
			//
			// While this is a bit scary, we have several processes to make
			// sure this never leaks to the actual container (the SET_DUMPABLE
			// bit, setting everything to O_CLOEXEC at the end of init, etc) --
			// and in addition, OPEN_TREE_CLONE file descriptors are completely
			// safe to leak to the container because they are in an anonymous
			// mount namespace so they cannot be used to escape the container
			// a-la CVE-2016-9962.
			cmd.ExtraFiles = append(cmd.ExtraFiles, mountFile)
			fd := stdioFdCount + len(cmd.ExtraFiles) - 1
			logrus.Debugf("remapping mount source %s to fd %d", m.Source, fd)
			c.config.Mounts[i].SourceFd = &fd
			// Prepend \x00 to m.Source to make sure that attempts to operate
			// on it in rootfs_linux.go fail.
			c.config.Mounts[i].Source = "\x00" + m.Source
		}
	}
	return nil
}

func (c *Container) newInitProcess(p *Process, cmd *exec.Cmd, messageSockPair, logFilePair filePair) (*initProcess, error) {
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE="+string(initStandard))
	nsMaps := make(map[configs.NamespaceType]string)
	for _, ns := range c.config.Namespaces {
		if ns.Path != "" {
			nsMaps[ns.Type] = ns.Path
		}
	}
	if err := c.remapMountSources(cmd); err != nil {
		return nil, err
	}
	data, err := c.bootstrapData(c.config.Namespaces.CloneFlags(), nsMaps)
	if err != nil {
		return nil, err
	}

	init := &initProcess{
		cmd:             cmd,
		messageSockPair: messageSockPair,
		logFilePair:     logFilePair,
		manager:         c.cgroupManager,
		intelRdtManager: c.intelRdtManager,
		config:          c.newInitConfig(p),
		container:       c,
		process:         p,
		bootstrapData:   data,
	}
	c.initProcess = init
	return init, nil
}

func (c *Container) newSetnsProcess(p *Process, cmd *exec.Cmd, messageSockPair, logFilePair filePair) (*setnsProcess, error) {
	cmd.Env = append(cmd.Env, "_LIBCONTAINER_INITTYPE="+string(initSetns))
	state, err := c.currentState()
	if err != nil {
		return nil, fmt.Errorf("unable to get container state: %w", err)
	}
	// for setns process, we don't have to set cloneflags as the process namespaces
	// will only be set via setns syscall
	data, err := c.bootstrapData(0, state.NamespacePaths)
	if err != nil {
		return nil, err
	}
	proc := &setnsProcess{
		cmd:             cmd,
		cgroupPaths:     state.CgroupPaths,
		rootlessCgroups: c.config.RootlessCgroups,
		intelRdtPath:    state.IntelRdtPath,
		messageSockPair: messageSockPair,
		logFilePair:     logFilePair,
		manager:         c.cgroupManager,
		config:          c.newInitConfig(p),
		process:         p,
		bootstrapData:   data,
		initProcessPid:  state.InitProcessPid,
	}
	if len(p.SubCgroupPaths) > 0 {
		if add, ok := p.SubCgroupPaths[""]; ok {
			// cgroup v1: using the same path for all controllers.
			// cgroup v2: the only possible way.
			for k := range proc.cgroupPaths {
				subPath := path.Join(proc.cgroupPaths[k], add)
				if !strings.HasPrefix(subPath, proc.cgroupPaths[k]) {
					return nil, fmt.Errorf("%s is not a sub cgroup path", add)
				}
				proc.cgroupPaths[k] = subPath
			}
			// cgroup v2: do not try to join init process's cgroup
			// as a fallback (see (*setnsProcess).start).
			proc.initProcessPid = 0
		} else {
			// Per-controller paths.
			for ctrl, add := range p.SubCgroupPaths {
				if val, ok := proc.cgroupPaths[ctrl]; ok {
					subPath := path.Join(val, add)
					if !strings.HasPrefix(subPath, val) {
						return nil, fmt.Errorf("%s is not a sub cgroup path", add)
					}
					proc.cgroupPaths[ctrl] = subPath
				} else {
					return nil, fmt.Errorf("unknown controller %s in SubCgroupPaths", ctrl)
				}
			}
		}
	}
	return proc, nil
}

func (c *Container) newInitConfig(process *Process) *initConfig {
	cfg := &initConfig{
		Config:           c.config,
		Args:             process.Args,
		Env:              process.Env,
		User:             process.User,
		AdditionalGroups: process.AdditionalGroups,
		Cwd:              process.Cwd,
		Capabilities:     process.Capabilities,
		PassedFilesCount: len(process.ExtraFiles),
		ContainerID:      c.ID(),
		NoNewPrivileges:  c.config.NoNewPrivileges,
		RootlessEUID:     c.config.RootlessEUID,
		RootlessCgroups:  c.config.RootlessCgroups,
		AppArmorProfile:  c.config.AppArmorProfile,
		ProcessLabel:     c.config.ProcessLabel,
		Rlimits:          c.config.Rlimits,
		CreateConsole:    process.ConsoleSocket != nil,
		ConsoleWidth:     process.ConsoleWidth,
		ConsoleHeight:    process.ConsoleHeight,
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
	return c.state.destroy()
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
		if err := c.cgroupManager.Freeze(configs.Frozen); err != nil {
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
	if err := c.cgroupManager.Freeze(configs.Thawed); err != nil {
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
	state, err := c.currentState()
	if err != nil {
		return nil, err
	}
	err = c.saveState(state)
	if err != nil {
		return nil, err
	}
	return state, nil
}

func (c *Container) saveState(s *State) (retErr error) {
	tmpFile, err := os.CreateTemp(c.root, "state-")
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

	stateFilePath := filepath.Join(c.root, stateFilename)
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
	t := c.runType()
	switch t {
	case Created:
		return c.state.transition(&createdState{c: c})
	case Running:
		return c.state.transition(&runningState{c: c})
	}
	return c.state.transition(&stoppedState{c: c})
}

func (c *Container) runType() Status {
	if c.initProcess == nil {
		return Stopped
	}
	pid := c.initProcess.pid()
	stat, err := system.Stat(pid)
	if err != nil {
		return Stopped
	}
	if stat.StartTime != c.initProcessStartTime || stat.State == system.Zombie || stat.State == system.Dead {
		return Stopped
	}
	// We'll create exec fifo and blocking on it after container is created,
	// and delete it after start container.
	if _, err := os.Stat(filepath.Join(c.root, execFifoFilename)); err == nil {
		return Created
	}
	return Running
}

func (c *Container) isPaused() (bool, error) {
	state, err := c.cgroupManager.GetFreezerState()
	if err != nil {
		return false, err
	}
	return state == configs.Frozen, nil
}

func (c *Container) currentState() (*State, error) {
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
	return state, nil
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
				if path, err := execabs.LookPath("newuidmap"); err == nil {
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
				if path, err := execabs.LookPath("newgidmap"); err == nil {
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
		{ContainerID: 0, HostID: os.Getegid(), Size: 1},
	}
	return !reflect.DeepEqual(c.GIDMappings, gidMap)
}

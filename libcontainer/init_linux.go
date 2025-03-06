package libcontainer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"syscall"

	"github.com/containerd/console"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/capabilities"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/utils"
)

type initType string

const (
	initSetns    initType = "setns"
	initStandard initType = "standard"
)

type pid struct {
	Pid           int `json:"stage2_pid"`
	PidFirstChild int `json:"stage1_pid"`
}

// network is an internal struct used to setup container networks.
type network struct {
	configs.Network

	// TempVethPeerName is a unique temporary veth peer name that was placed into
	// the container's namespace.
	TempVethPeerName string `json:"temp_veth_peer_name"`
}

// initConfig is used for transferring parameters from Exec() to Init().
// It contains:
//   - original container config;
//   - some [Process] properties;
//   - set of properties merged from the container config ([configs.Config])
//     and the process ([Process]);
//   - some properties that come from the container.
//
// When adding new fields, please make sure they go into the relevant section.
type initConfig struct {
	// Config is the original container config.
	Config *configs.Config `json:"config"`

	// Properties that are unique to and come from [Process].

	Args             []string `json:"args"`
	Env              []string `json:"env"`
	UID              int      `json:"uid"`
	GID              int      `json:"gid"`
	AdditionalGroups []int    `json:"additional_groups"`
	Cwd              string   `json:"cwd"`
	CreateConsole    bool     `json:"create_console"`
	ConsoleWidth     uint16   `json:"console_width"`
	ConsoleHeight    uint16   `json:"console_height"`
	PassedFilesCount int      `json:"passed_files_count"`

	// Properties that exists both in the container config and the process,
	// as merged by [Container.newInitConfig] (process properties has preference).

	AppArmorProfile string                `json:"apparmor_profile"`
	Capabilities    *configs.Capabilities `json:"capabilities"`
	NoNewPrivileges bool                  `json:"no_new_privileges"`
	ProcessLabel    string                `json:"process_label"`
	Rlimits         []configs.Rlimit      `json:"rlimits"`
	IOPriority      *configs.IOPriority   `json:"io_priority,omitempty"`
	Scheduler       *configs.Scheduler    `json:"scheduler,omitempty"`
	CPUAffinity     *configs.CPUAffinity  `json:"cpu_affinity,omitempty"`

	// Miscellaneous properties, filled in by [Container.newInitConfig]
	// unless documented otherwise.

	ContainerID string `json:"containerid"`
	Cgroup2Path string `json:"cgroup2_path,omitempty"`

	// Networks is filled in from container config by [initProcess.createNetworkInterfaces].
	Networks []*network `json:"network"`

	// SpecState is filled in by [initProcess.Start].
	SpecState *specs.State `json:"spec_state,omitempty"`
}

// Init is part of "runc init" implementation.
func Init() {
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()

	if err := startInitialization(); err != nil {
		// If the error is returned, it was not communicated
		// back to the parent (which is not a common case),
		// so print it to stderr here as a last resort.
		//
		// Do not use logrus as we are not sure if it has been
		// set up yet, but most important, if the parent is
		// alive (and its log forwarding is working).
		fmt.Fprintln(os.Stderr, err)
	}
	// Normally, StartInitialization() never returns, meaning
	// if we are here, it had failed.
	os.Exit(255)
}

// Normally, this function does not return. If it returns, with or without an
// error, it means the initialization has failed. If the error is returned,
// it means the error can not be communicated back to the parent.
func startInitialization() (retErr error) {
	// Get the synchronisation pipe.
	envSyncPipe := os.Getenv("_LIBCONTAINER_SYNCPIPE")
	syncPipeFd, err := strconv.Atoi(envSyncPipe)
	if err != nil {
		return fmt.Errorf("unable to convert _LIBCONTAINER_SYNCPIPE: %w", err)
	}
	syncPipe := newSyncSocket(os.NewFile(uintptr(syncPipeFd), "sync"))
	defer syncPipe.Close()

	defer func() {
		// If this defer is ever called, this means initialization has failed.
		// Send the error back to the parent process in the form of an initError
		// if the sync socket has not been closed.
		if syncPipe.isClosed() {
			return
		}
		ierr := initError{Message: retErr.Error()}
		if err := writeSyncArg(syncPipe, procError, ierr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		// The error is sent, no need to also return it (or it will be reported twice).
		retErr = nil
	}()

	// Get the INITPIPE.
	envInitPipe := os.Getenv("_LIBCONTAINER_INITPIPE")
	initPipeFd, err := strconv.Atoi(envInitPipe)
	if err != nil {
		return fmt.Errorf("unable to convert _LIBCONTAINER_INITPIPE: %w", err)
	}
	initPipe := os.NewFile(uintptr(initPipeFd), "init")
	defer initPipe.Close()

	// Set up logging. This is used rarely, and mostly for init debugging.

	// Passing log level is optional; currently libcontainer/integration does not do it.
	if levelStr := os.Getenv("_LIBCONTAINER_LOGLEVEL"); levelStr != "" {
		logLevel, err := strconv.Atoi(levelStr)
		if err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_LOGLEVEL: %w", err)
		}
		logrus.SetLevel(logrus.Level(logLevel))
	}

	logFd, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_LOGPIPE"))
	if err != nil {
		return fmt.Errorf("unable to convert _LIBCONTAINER_LOGPIPE: %w", err)
	}
	logPipe := os.NewFile(uintptr(logFd), "logpipe")

	logrus.SetOutput(logPipe)
	logrus.SetFormatter(new(logrus.JSONFormatter))
	logrus.Debug("child process in init()")

	// Only init processes have FIFOFD.
	var fifoFile *os.File
	envInitType := os.Getenv("_LIBCONTAINER_INITTYPE")
	it := initType(envInitType)
	if it == initStandard {
		fifoFd, err := strconv.Atoi(os.Getenv("_LIBCONTAINER_FIFOFD"))
		if err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_FIFOFD: %w", err)
		}
		fifoFile = os.NewFile(uintptr(fifoFd), "initfifo")
	}

	var consoleSocket *os.File
	if envConsole := os.Getenv("_LIBCONTAINER_CONSOLE"); envConsole != "" {
		console, err := strconv.Atoi(envConsole)
		if err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_CONSOLE: %w", err)
		}
		consoleSocket = os.NewFile(uintptr(console), "console-socket")
		defer consoleSocket.Close()
	}

	var pidfdSocket *os.File
	if envSockFd := os.Getenv("_LIBCONTAINER_PIDFD_SOCK"); envSockFd != "" {
		sockFd, err := strconv.Atoi(envSockFd)
		if err != nil {
			return fmt.Errorf("unable to convert _LIBCONTAINER_PIDFD_SOCK: %w", err)
		}
		pidfdSocket = os.NewFile(uintptr(sockFd), "pidfd-socket")
		defer pidfdSocket.Close()
	}

	// From here on, we don't need current process environment. It is not
	// used directly anywhere below this point, but let's clear it anyway.
	os.Clearenv()

	defer func() {
		if err := recover(); err != nil {
			if err2, ok := err.(error); ok {
				retErr = fmt.Errorf("panic from initialization: %w, %s", err2, debug.Stack())
			} else {
				retErr = fmt.Errorf("panic from initialization: %v, %s", err, debug.Stack())
			}
		}
	}()

	var config initConfig
	if err := json.NewDecoder(initPipe).Decode(&config); err != nil {
		return err
	}

	// If init succeeds, it will not return, hence none of the defers will be called.
	return containerInit(it, &config, syncPipe, consoleSocket, pidfdSocket, fifoFile, logPipe)
}

func containerInit(t initType, config *initConfig, pipe *syncSocket, consoleSocket, pidfdSocket, fifoFile, logPipe *os.File) error {
	// Clean the RLIMIT_NOFILE cache in go runtime.
	// Issue: https://github.com/opencontainers/runc/issues/4195
	maybeClearRlimitNofileCache(config.Rlimits)

	switch t {
	case initSetns:
		i := &linuxSetnsInit{
			pipe:          pipe,
			consoleSocket: consoleSocket,
			pidfdSocket:   pidfdSocket,
			config:        config,
			logPipe:       logPipe,
		}
		return i.Init()
	case initStandard:
		i := &linuxStandardInit{
			pipe:          pipe,
			consoleSocket: consoleSocket,
			pidfdSocket:   pidfdSocket,
			parentPid:     unix.Getppid(),
			config:        config,
			fifoFile:      fifoFile,
			logPipe:       logPipe,
		}
		return i.Init()
	}
	return fmt.Errorf("unknown init type %q", t)
}

// verifyCwd ensures that the current directory is actually inside the mount
// namespace root of the current process.
func verifyCwd() error {
	// getcwd(2) on Linux detects if cwd is outside of the rootfs of the
	// current mount namespace root, and in that case prefixes "(unreachable)"
	// to the returned string. glibc's getcwd(3) and Go's Getwd() both detect
	// when this happens and return ENOENT rather than returning a non-absolute
	// path. In both cases we can therefore easily detect if we have an invalid
	// cwd by checking the return value of getcwd(3). See getcwd(3) for more
	// details, and CVE-2024-21626 for the security issue that motivated this
	// check.
	//
	// We have to use unix.Getwd() here because os.Getwd() has a workaround for
	// $PWD which involves doing stat(.), which can fail if the current
	// directory is inaccessible to the container process.
	if wd, err := unix.Getwd(); errors.Is(err, unix.ENOENT) {
		return errors.New("current working directory is outside of container mount namespace root -- possible container breakout detected")
	} else if err != nil {
		return fmt.Errorf("failed to verify if current working directory is safe: %w", err)
	} else if !filepath.IsAbs(wd) {
		// We shouldn't ever hit this, but check just in case.
		return fmt.Errorf("current working directory is not absolute -- possible container breakout detected: cwd is %q", wd)
	}
	return nil
}

// finalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaked file descriptors
// before executing the command inside the namespace.
func finalizeNamespace(config *initConfig) error {
	// Ensure that all unwanted fds we may have accidentally
	// inherited are marked close-on-exec so they stay out of the
	// container
	if err := utils.CloseExecFrom(config.PassedFilesCount + 3); err != nil {
		return fmt.Errorf("error closing exec fds: %w", err)
	}

	// we only do chdir if it's specified
	doChdir := config.Cwd != ""
	if doChdir {
		// First, attempt the chdir before setting up the user.
		// This could allow us to access a directory that the user running runc can access
		// but the container user cannot.
		err := unix.Chdir(config.Cwd)
		switch {
		case err == nil:
			doChdir = false
		case os.IsPermission(err):
			// If we hit an EPERM, we should attempt again after setting up user.
			// This will allow us to successfully chdir if the container user has access
			// to the directory, but the user running runc does not.
			// This is useful in cases where the cwd is also a volume that's been chowned to the container user.
		default:
			return fmt.Errorf("chdir to cwd (%q) set in config.json failed: %w", config.Cwd, err)
		}
	}

	// We should set envs after we are in the jail of the container.
	// Please see https://github.com/opencontainers/runc/issues/4688
	env, err := prepareEnv(config.Env, config.UID)
	if err != nil {
		return err
	}
	config.Env = env

	w, err := capabilities.New(config.Capabilities)
	if err != nil {
		return err
	}
	// drop capabilities in bounding set before changing user
	if err := w.ApplyBoundingSet(); err != nil {
		return fmt.Errorf("unable to apply bounding set: %w", err)
	}
	// preserve existing capabilities while we change users
	if err := system.SetKeepCaps(); err != nil {
		return fmt.Errorf("unable to set keep caps: %w", err)
	}
	if err := setupUser(config); err != nil {
		return fmt.Errorf("unable to setup user: %w", err)
	}
	// Change working directory AFTER the user has been set up, if we haven't done it yet.
	if doChdir {
		if err := unix.Chdir(config.Cwd); err != nil {
			return fmt.Errorf("chdir to cwd (%q) set in config.json failed: %w", config.Cwd, err)
		}
	}
	// Make sure our final working directory is inside the container.
	if err := verifyCwd(); err != nil {
		return err
	}
	if err := system.ClearKeepCaps(); err != nil {
		return fmt.Errorf("unable to clear keep caps: %w", err)
	}
	if err := w.ApplyCaps(); err != nil {
		return fmt.Errorf("unable to apply caps: %w", err)
	}
	return nil
}

// setupConsole sets up the console from inside the container, and sends the
// master pty fd to the config.Pipe (using cmsg). This is done to ensure that
// consoles are scoped to a container properly (see runc#814 and the many
// issues related to that). This has to be run *after* we've pivoted to the new
// rootfs (and the users' configuration is entirely set up).
func setupConsole(socket *os.File, config *initConfig, mount bool) error {
	defer socket.Close()
	// At this point, /dev/ptmx points to something that we would expect. We
	// used to change the owner of the slave path, but since the /dev/pts mount
	// can have gid=X set (at the users' option). So touching the owner of the
	// slave PTY is not necessary, as the kernel will handle that for us. Note
	// however, that setupUser (specifically fixStdioPermissions) *will* change
	// the UID owner of the console to be the user the process will run as (so
	// they can actually control their console).

	pty, slavePath, err := console.NewPty()
	if err != nil {
		return err
	}
	// After we return from here, we don't need the console anymore.
	defer pty.Close()

	if config.ConsoleHeight != 0 && config.ConsoleWidth != 0 {
		err = pty.Resize(console.WinSize{
			Height: config.ConsoleHeight,
			Width:  config.ConsoleWidth,
		})
		if err != nil {
			return err
		}
	}

	// Mount the console inside our rootfs.
	if mount {
		if err := mountConsole(slavePath); err != nil {
			return err
		}
	}
	// While we can access console.master, using the API is a good idea.
	if err := utils.SendRawFd(socket, pty.Name(), pty.Fd()); err != nil {
		return err
	}
	runtime.KeepAlive(pty)

	// Now, dup over all the things.
	return dupStdio(slavePath)
}

// syncParentReady sends to the given pipe a JSON payload which indicates that
// the init is ready to Exec the child process. It then waits for the parent to
// indicate that it is cleared to Exec.
func syncParentReady(pipe *syncSocket) error {
	// Tell parent.
	if err := writeSync(pipe, procReady); err != nil {
		return err
	}
	// Wait for parent to give the all-clear.
	return readSync(pipe, procRun)
}

// syncParentHooks sends to the given pipe a JSON payload which indicates that
// the parent should execute pre-start hooks. It then waits for the parent to
// indicate that it is cleared to resume.
func syncParentHooks(pipe *syncSocket) error {
	// Tell parent.
	if err := writeSync(pipe, procHooks); err != nil {
		return err
	}
	// Wait for parent to give the all-clear.
	return readSync(pipe, procHooksDone)
}

// syncParentSeccomp sends the fd associated with the seccomp file descriptor
// to the parent, and wait for the parent to do pidfd_getfd() to grab a copy.
func syncParentSeccomp(pipe *syncSocket, seccompFd int) error {
	if seccompFd == -1 {
		return nil
	}
	defer unix.Close(seccompFd)

	// Tell parent to grab our fd.
	//
	// Notably, we do not use writeSyncFile here because a container might have
	// an SCMP_ACT_NOTIFY action on sendmsg(2) so we need to use the smallest
	// possible number of system calls here because all of those syscalls
	// cannot be used with SCMP_ACT_NOTIFY as a result (any syscall we use here
	// before the parent gets the file descriptor would deadlock "runc init" if
	// we allowed it for SCMP_ACT_NOTIFY). See seccomp.InitSeccomp() for more
	// details.
	if err := writeSyncArg(pipe, procSeccomp, seccompFd); err != nil {
		return err
	}
	// Wait for parent to tell us they've grabbed the seccompfd.
	return readSync(pipe, procSeccompDone)
}

// setupUser changes the groups, gid, and uid for the user inside the container.
func setupUser(config *initConfig) error {
	// Before we change to the container's user make sure that the processes
	// STDIO is correctly owned by the user that we are switching to.
	if err := fixStdioPermissions(config.UID); err != nil {
		return err
	}

	// We don't need to use /proc/thread-self here because setgroups is a
	// per-userns file and thus is global to all threads in a thread-group.
	// This lets us avoid having to do runtime.LockOSThread.
	setgroups, err := os.ReadFile("/proc/self/setgroups")
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// This isn't allowed in an unprivileged user namespace since Linux 3.19.
	// There's nothing we can do about /etc/group entries, so we silently
	// ignore setting groups here (since the user didn't explicitly ask us to
	// set the group).
	allowSupGroups := !config.Config.RootlessEUID && string(bytes.TrimSpace(setgroups)) != "deny"

	if allowSupGroups {
		if err := unix.Setgroups(config.AdditionalGroups); err != nil {
			return &os.SyscallError{Syscall: "setgroups", Err: err}
		}
	}

	if err := unix.Setgid(config.GID); err != nil {
		if err == unix.EINVAL {
			return fmt.Errorf("cannot setgid to unmapped gid %d in user namespace", config.GID)
		}
		return err
	}
	if err := unix.Setuid(config.UID); err != nil {
		if err == unix.EINVAL {
			return fmt.Errorf("cannot setuid to unmapped uid %d in user namespace", config.UID)
		}
		return err
	}
	return nil
}

// fixStdioPermissions fixes the permissions of PID 1's STDIO within the container to the specified uid.
// The ownership needs to match because it is created outside of the container and needs to be
// localized.
func fixStdioPermissions(uid int) error {
	for _, file := range []*os.File{os.Stdin, os.Stdout, os.Stderr} {
		var s unix.Stat_t
		if err := unix.Fstat(int(file.Fd()), &s); err != nil {
			return &os.PathError{Op: "fstat", Path: file.Name(), Err: err}
		}

		// Skip chown if:
		// - uid is already the one we want, or
		// - fd is opened to /dev/null.
		if int(s.Uid) == uid || isDevNull(&s) {
			continue
		}

		// We only change the uid (as it is possible for the mount to
		// prefer a different gid, and there's no reason for us to change it).
		// The reason why we don't just leave the default uid=X mount setup is
		// that users expect to be able to actually use their console. Without
		// this code, you couldn't effectively run as a non-root user inside a
		// container and also have a console set up.
		if err := file.Chown(uid, int(s.Gid)); err != nil {
			// If we've hit an EINVAL then s.Gid isn't mapped in the user
			// namespace. If we've hit an EPERM then the inode's current owner
			// is not mapped in our user namespace (in particular,
			// privileged_wrt_inode_uidgid() has failed). Read-only
			// /dev can result in EROFS error. In any case, it's
			// better for us to just not touch the stdio rather
			// than bail at this point.

			if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EROFS) {
				continue
			}
			return err
		}
	}
	return nil
}

// setupNetwork sets up and initializes any network interface inside the container.
func setupNetwork(config *initConfig) error {
	for _, config := range config.Networks {
		strategy, err := getStrategy(config.Type)
		if err != nil {
			return err
		}
		if err := strategy.initialize(config); err != nil {
			return err
		}
	}
	return nil
}

func setupRoute(config *configs.Config) error {
	for _, config := range config.Routes {
		_, dst, err := net.ParseCIDR(config.Destination)
		if err != nil {
			return err
		}
		src := net.ParseIP(config.Source)
		if src == nil {
			return fmt.Errorf("Invalid source for route: %s", config.Source)
		}
		gw := net.ParseIP(config.Gateway)
		if gw == nil {
			return fmt.Errorf("Invalid gateway for route: %s", config.Gateway)
		}
		l, err := netlink.LinkByName(config.InterfaceName)
		if err != nil {
			return err
		}
		route := &netlink.Route{
			Scope:     netlink.SCOPE_UNIVERSE,
			Dst:       dst,
			Src:       src,
			Gw:        gw,
			LinkIndex: l.Attrs().Index,
		}
		if err := netlink.RouteAdd(route); err != nil {
			return err
		}
	}
	return nil
}

func maybeClearRlimitNofileCache(limits []configs.Rlimit) {
	for _, rlimit := range limits {
		if rlimit.Type == syscall.RLIMIT_NOFILE {
			system.ClearRlimitNofileCache(&syscall.Rlimit{
				Cur: rlimit.Soft,
				Max: rlimit.Hard,
			})
			return
		}
	}
}

func setupRlimits(limits []configs.Rlimit, pid int) error {
	for _, rlimit := range limits {
		if err := unix.Prlimit(pid, rlimit.Type, &unix.Rlimit{Max: rlimit.Hard, Cur: rlimit.Soft}, nil); err != nil {
			return fmt.Errorf("error setting rlimit type %v: %w", rlimit.Type, err)
		}
	}
	return nil
}

func setupScheduler(config *initConfig) error {
	if config.Scheduler == nil {
		return nil
	}
	attr, err := configs.ToSchedAttr(config.Scheduler)
	if err != nil {
		return err
	}
	if err := unix.SchedSetAttr(0, attr, 0); err != nil {
		if errors.Is(err, unix.EPERM) && config.Config.Cgroups.CpusetCpus != "" {
			return errors.New("process scheduler can't be used together with AllowedCPUs")
		}
		return fmt.Errorf("error setting scheduler: %w", err)
	}
	return nil
}

func setupIOPriority(config *initConfig) error {
	const ioprioWhoPgrp = 1

	ioprio := config.IOPriority
	if ioprio == nil {
		return nil
	}
	class := 0
	switch ioprio.Class {
	case specs.IOPRIO_CLASS_RT:
		class = 1
	case specs.IOPRIO_CLASS_BE:
		class = 2
	case specs.IOPRIO_CLASS_IDLE:
		class = 3
	default:
		return fmt.Errorf("invalid io priority class: %s", ioprio.Class)
	}

	// Combine class and priority into a single value
	// https://github.com/torvalds/linux/blob/v5.18/include/uapi/linux/ioprio.h#L5-L17
	iop := (class << 13) | ioprio.Priority
	_, _, errno := unix.RawSyscall(unix.SYS_IOPRIO_SET, ioprioWhoPgrp, 0, uintptr(iop))
	if errno != 0 {
		return fmt.Errorf("failed to set io priority: %w", errno)
	}
	return nil
}

func setupPersonality(config *configs.Config) error {
	return system.SetLinuxPersonality(config.Personality.Domain)
}

// signalAllProcesses freezes then iterates over all the processes inside the
// manager's cgroups sending the signal s to them.
func signalAllProcesses(m cgroups.Manager, s unix.Signal) error {
	if !m.Exists() {
		return ErrCgroupNotExist
	}
	// Use cgroup.kill, if available.
	if s == unix.SIGKILL {
		if p := m.Path(""); p != "" { // Either cgroup v2 or hybrid.
			err := cgroups.WriteFile(p, "cgroup.kill", "1")
			if err == nil || !errors.Is(err, os.ErrNotExist) {
				return err
			}
			// Fallback to old implementation.
		}
	}

	if err := m.Freeze(cgroups.Frozen); err != nil {
		logrus.Warn(err)
	}
	pids, err := m.GetAllPids()
	if err != nil {
		if err := m.Freeze(cgroups.Thawed); err != nil {
			logrus.Warn(err)
		}
		return err
	}
	for _, pid := range pids {
		err := unix.Kill(pid, s)
		if err != nil && err != unix.ESRCH {
			logrus.Warnf("kill %d: %v", pid, err)
		}
	}
	if err := m.Freeze(cgroups.Thawed); err != nil {
		logrus.Warn(err)
	}

	return nil
}

// setupPidfd opens a process file descriptor of init process, and sends the
// file descriptor back to the socket.
func setupPidfd(socket *os.File, initType string) error {
	defer socket.Close()

	pidFd, err := unix.PidfdOpen(os.Getpid(), 0)
	if err != nil {
		return fmt.Errorf("failed to pidfd_open: %w", err)
	}

	if err := utils.SendRawFd(socket, initType, uintptr(pidFd)); err != nil {
		unix.Close(pidFd)
		return fmt.Errorf("failed to send pidfd on socket: %w", err)
	}
	return unix.Close(pidFd)
}

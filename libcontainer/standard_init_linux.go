package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/keys"
	"github.com/opencontainers/runc/libcontainer/seccomp"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/opencontainers/runc/libcontainer/utils"
)

type linuxStandardInit struct {
	pipe          *syncSocket
	consoleSocket *os.File
	pidfdSocket   *os.File
	parentPid     int
	fifoFile      *os.File
	logPipe       *os.File
	config        *initConfig
}

func (l *linuxStandardInit) getSessionRingParams() (string, uint32, uint32) {
	var newperms uint32

	if l.config.Config.Namespaces.Contains(configs.NEWUSER) {
		// With user ns we need 'other' search permissions.
		newperms = 0x8
	} else {
		// Without user ns we need 'UID' search permissions.
		newperms = 0x80000
	}

	// Create a unique per session container name that we can join in setns;
	// However, other containers can also join it.
	return "_ses." + l.config.ContainerID, 0xffffffff, newperms
}

func (l *linuxStandardInit) Init() error {
	if !l.config.Config.NoNewKeyring {
		if l.config.ProcessLabel != "" {
			if err := selinux.SetKeyLabel(l.config.ProcessLabel); err != nil {
				return err
			}
			defer selinux.SetKeyLabel("") //nolint: errcheck
		}
		ringname, keepperms, newperms := l.getSessionRingParams()

		// Do not inherit the parent's session keyring.
		if sessKeyId, err := keys.JoinSessionKeyring(ringname); err != nil {
			logrus.Warnf("KeyctlJoinSessionKeyring: %v", err)
			// If keyrings aren't supported then it is likely we are on an
			// older kernel (or inside an LXC container). While we could bail,
			// the security feature we are using here is best-effort (it only
			// really provides marginal protection since VFS credentials are
			// the only significant protection of keyrings).
			if !errors.Is(err, unix.ENOSYS) {
				return fmt.Errorf("unable to join session keyring: %w", err)
			}
		} else {
			// Make session keyring searchable. If we've gotten this far we
			// bail on any error -- we don't want to have a keyring with bad
			// permissions.
			if err := keys.ModKeyringPerm(sessKeyId, keepperms, newperms); err != nil {
				return fmt.Errorf("unable to mod keyring permissions: %w", err)
			}
		}
	}

	if err := setupNetwork(l.config); err != nil {
		return err
	}
	if err := setupRoute(l.config.Config); err != nil {
		return err
	}

	// initialises the labeling system
	selinux.GetEnabled()

	err := prepareRootfs(l.pipe, l.config)
	if err != nil {
		return err
	}

	// Set up the console. This has to be done *before* we finalize the rootfs,
	// but *after* we've given the user the chance to set up all of the mounts
	// they wanted.
	if l.config.CreateConsole {
		if err := setupConsole(l.consoleSocket, l.config, true); err != nil {
			return err
		}
		if err := system.Setctty(); err != nil {
			return &os.SyscallError{Syscall: "ioctl(setctty)", Err: err}
		}
	}

	if l.pidfdSocket != nil {
		if err := setupPidfd(l.pidfdSocket, "standard"); err != nil {
			return fmt.Errorf("failed to setup pidfd: %w", err)
		}
	}

	// Finish the rootfs setup.
	if l.config.Config.Namespaces.Contains(configs.NEWNS) {
		if err := finalizeRootfs(l.config.Config); err != nil {
			return err
		}
	}

	if hostname := l.config.Config.Hostname; hostname != "" {
		if err := unix.Sethostname([]byte(hostname)); err != nil {
			return &os.SyscallError{Syscall: "sethostname", Err: err}
		}
	}
	if domainname := l.config.Config.Domainname; domainname != "" {
		if err := unix.Setdomainname([]byte(domainname)); err != nil {
			return &os.SyscallError{Syscall: "setdomainname", Err: err}
		}
	}
	if err := apparmor.ApplyProfile(l.config.AppArmorProfile); err != nil {
		return fmt.Errorf("unable to apply apparmor profile: %w", err)
	}

	for key, value := range l.config.Config.Sysctl {
		if err := writeSystemProperty(key, value); err != nil {
			return err
		}
	}
	for _, path := range l.config.Config.ReadonlyPaths {
		if err := readonlyPath(path); err != nil {
			return fmt.Errorf("can't make %q read-only: %w", path, err)
		}
	}
	for _, path := range l.config.Config.MaskPaths {
		if err := maskPath(path, l.config.Config.MountLabel); err != nil {
			return fmt.Errorf("can't mask path %s: %w", path, err)
		}
	}
	pdeath, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("can't get pdeath signal: %w", err)
	}
	if l.config.NoNewPrivileges {
		if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
			return &os.SyscallError{Syscall: "prctl(SET_NO_NEW_PRIVS)", Err: err}
		}
	}

	if err := setupScheduler(l.config); err != nil {
		return err
	}

	if err := setupIOPriority(l.config); err != nil {
		return err
	}

	// Set personality if specified.
	if l.config.Config.Personality != nil {
		if err := setupPersonality(l.config.Config); err != nil {
			return err
		}
	}

	if err := setupMemoryPolicy(l.config.Config); err != nil {
		return err
	}

	// Tell our parent that we're ready to exec. This must be done before the
	// Seccomp rules have been applied, because we need to be able to read and
	// write to a socket.
	if err := syncParentReady(l.pipe); err != nil {
		return fmt.Errorf("sync ready: %w", err)
	}
	if l.config.ProcessLabel != "" {
		if err := selinux.SetExecLabel(l.config.ProcessLabel); err != nil {
			return fmt.Errorf("can't set process label: %w", err)
		}
		defer selinux.SetExecLabel("") //nolint: errcheck
	}
	// Without NoNewPrivileges seccomp is a privileged operation, so we need to
	// do this before dropping capabilities; otherwise do it as late as possible
	// just before execve so as few syscalls take place after it as possible.
	if l.config.Config.Seccomp != nil && !l.config.NoNewPrivileges {
		seccompFd, err := seccomp.InitSeccomp(l.config.Config.Seccomp)
		if err != nil {
			return err
		}

		if err := syncParentSeccomp(l.pipe, seccompFd); err != nil {
			return err
		}
	}
	if err := finalizeNamespace(l.config); err != nil {
		return err
	}
	// finalizeNamespace can change user/group which clears the parent death
	// signal, so we restore it here.
	if err := pdeath.Restore(); err != nil {
		return fmt.Errorf("can't restore pdeath signal: %w", err)
	}

	// In case we have any StartContainer hooks to run, and they don't
	// have environment configured explicitly, make sure they will be run
	// with the same environment as container's init.
	//
	// NOTE the above described behavior is not part of runtime-spec, but
	// rather a de facto historical thing we afraid to change.
	if h := l.config.Config.Hooks[configs.StartContainer]; len(h) > 0 {
		h.SetDefaultEnv(l.config.Env)
	}

	// Compare the parent from the initial start of the init process and make
	// sure that it did not change.  if the parent changes that means it died
	// and we were reparented to something else so we should just kill ourself
	// and not cause problems for someone else.
	if unix.Getppid() != l.parentPid {
		return unix.Kill(unix.Getpid(), unix.SIGKILL)
	}
	// Check for the arg before waiting to make sure it exists and it is
	// returned as a create time error.
	name, err := exec.LookPath(l.config.Args[0])
	if err != nil {
		return err
	}

	// Set seccomp as close to execve as possible, so as few syscalls take
	// place afterward (reducing the amount of syscalls that users need to
	// enable in their seccomp profiles). However, this needs to be done
	// before closing the pipe since we need it to pass the seccompFd to
	// the parent.
	if l.config.Config.Seccomp != nil && l.config.NoNewPrivileges {
		seccompFd, err := seccomp.InitSeccomp(l.config.Config.Seccomp)
		if err != nil {
			return fmt.Errorf("unable to init seccomp: %w", err)
		}

		if err := syncParentSeccomp(l.pipe, seccompFd); err != nil {
			return err
		}
	}

	// Close the pipe to signal that we have completed our init.
	logrus.Debugf("init: closing the pipe to signal completion")
	_ = l.pipe.Close()

	// Close the log pipe fd so the parent's ForwardLogs can exit.
	logrus.Debugf("init: about to wait on exec fifo")
	if err := l.logPipe.Close(); err != nil {
		return fmt.Errorf("close log pipe: %w", err)
	}

	fifoPath, closer := utils.ProcThreadSelfFd(l.fifoFile.Fd())
	defer closer()

	// Wait for the FIFO to be opened on the other side before exec-ing the
	// user process. We open it through /proc/self/fd/$fd, because the fd that
	// was given to us was an O_PATH fd to the fifo itself. Linux allows us to
	// re-open an O_PATH fd through /proc.
	fd, err := linux.Open(fifoPath, unix.O_WRONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	if _, err := unix.Write(fd, []byte("0")); err != nil {
		return &os.PathError{Op: "write exec fifo", Path: fifoPath, Err: err}
	}

	// Close the O_PATH fifofd fd before exec because the kernel resets
	// dumpable in the wrong order. This has been fixed in newer kernels, but
	// we keep this to ensure CVE-2016-9962 doesn't re-emerge on older kernels.
	// N.B. the core issue itself (passing dirfds to the host filesystem) has
	// since been resolved.
	// https://github.com/torvalds/linux/blob/v4.9/fs/exec.c#L1290-L1318
	_ = l.fifoFile.Close()

	if s := l.config.SpecState; s != nil {
		s.Pid = unix.Getpid()
		s.Status = specs.StateCreated
		if err := l.config.Config.Hooks.Run(configs.StartContainer, s); err != nil {
			return err
		}
	}

	// Close all file descriptors we are not passing to the container. This is
	// necessary because the execve target could use internal runc fds as the
	// execve path, potentially giving access to binary files from the host
	// (which can then be opened by container processes, leading to container
	// escapes). Note that because this operation will close any open file
	// descriptors that are referenced by (*os.File) handles from underneath
	// the Go runtime, we must not do any file operations after this point
	// (otherwise the (*os.File) finaliser could close the wrong file). See
	// CVE-2024-21626 for more information as to why this protection is
	// necessary.
	if err := utils.UnsafeCloseFrom(l.config.PassedFilesCount + 3); err != nil {
		return err
	}
	return linux.Exec(name, l.config.Args, l.config.Env)
}

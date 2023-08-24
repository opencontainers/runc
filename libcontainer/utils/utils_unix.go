//go:build !windows
// +build !windows

package utils

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// EnsureProcHandle returns whether or not the given file handle is on procfs.
func EnsureProcHandle(fh *os.File) error {
	var buf unix.Statfs_t
	if err := unix.Fstatfs(int(fh.Fd()), &buf); err != nil {
		return fmt.Errorf("ensure %s is on procfs: %w", fh.Name(), err)
	}
	if buf.Type != unix.PROC_SUPER_MAGIC {
		return fmt.Errorf("%s is not on procfs", fh.Name())
	}
	return nil
}

var (
	haveCloseRangeCloexecBool bool
	haveCloseRangeCloexecOnce sync.Once
)

func haveCloseRangeCloexec() bool {
	haveCloseRangeCloexecOnce.Do(func() {
		// Make sure we're not closing a random file descriptor.
		tmpFd, err := unix.FcntlInt(0, unix.F_DUPFD_CLOEXEC, 0)
		if err != nil {
			return
		}
		defer unix.Close(tmpFd)

		err = unix.CloseRange(uint(tmpFd), uint(tmpFd), unix.CLOSE_RANGE_CLOEXEC)
		// Any error means we cannot use close_range(CLOSE_RANGE_CLOEXEC).
		// -ENOSYS and -EINVAL ultimately mean we don't have support, but any
		// other potential error would imply that even the most basic close
		// operation wouldn't work.
		haveCloseRangeCloexecBool = err == nil
	})
	return haveCloseRangeCloexecBool
}

// CloseExecFrom applies O_CLOEXEC to all file descriptors currently open for
// the process (except for those below the given fd value).
func CloseExecFrom(minFd int) error {
	if haveCloseRangeCloexec() {
		err := unix.CloseRange(uint(minFd), math.MaxUint, unix.CLOSE_RANGE_CLOEXEC)
		return os.NewSyscallError("close_range", err)
	}

	procSelfFd, closer := ProcThreadSelf("fd")
	defer closer()

	fdDir, err := os.Open(procSelfFd)
	if err != nil {
		return err
	}
	defer fdDir.Close()

	if err := EnsureProcHandle(fdDir); err != nil {
		return err
	}

	fdList, err := fdDir.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, fdStr := range fdList {
		fd, err := strconv.Atoi(fdStr)
		// Ignore non-numeric file names.
		if err != nil {
			continue
		}
		// Ignore descriptors lower than our specified minimum.
		if fd < minFd {
			continue
		}
		// Intentionally ignore errors from unix.CloseOnExec -- the cases where
		// this might fail are basically file descriptors that have already
		// been closed (including and especially the one that was created when
		// os.ReadDir did the "opendir" syscall).
		unix.CloseOnExec(fd)
	}
	return nil
}

// NewSockPair returns a new SOCK_STREAM unix socket pair.
func NewSockPair(name string) (parent, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), name+"-p"), os.NewFile(uintptr(fds[0]), name+"-c"), nil
}

// WithProcfd runs the passed closure with a procfd path (/proc/self/fd/...)
// corresponding to the unsafePath resolved within the root. Before passing the
// fd, this path is verified to have been inside the root -- so operating on it
// through the passed fdpath should be safe. Do not access this path through
// the original path strings, and do not attempt to use the pathname outside of
// the passed closure (the file handle will be freed once the closure returns).
func WithProcfd(root, unsafePath string, fn func(procfd string) error) error {
	// Remove the root then forcefully resolve inside the root.
	unsafePath = stripRoot(root, unsafePath)
	path, err := securejoin.SecureJoin(root, unsafePath)
	if err != nil {
		return fmt.Errorf("resolving path inside rootfs failed: %w", err)
	}

	procSelfFd, closer := ProcThreadSelf("fd/")
	defer closer()

	// Open the target path.
	fh, err := os.OpenFile(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open o_path procfd: %w", err)
	}
	defer fh.Close()

	procfd := filepath.Join(procSelfFd, strconv.Itoa(int(fh.Fd())))
	// Double-check the path is the one we expected.
	if realpath, err := os.Readlink(procfd); err != nil {
		return fmt.Errorf("procfd verification failed: %w", err)
	} else if realpath != path {
		return fmt.Errorf("possibly malicious path detected -- refusing to operate on %s", realpath)
	}

	return fn(procfd)
}

type ProcThreadSelfCloser func()

var (
	haveProcThreadSelf     bool
	haveProcThreadSelfOnce sync.Once
)

// ProcThreadSelf returns a string that is equivalent to
// /proc/thread-self/<subpath>, with a graceful fallback on older kernels where
// /proc/thread-self doesn't exist. This method DOES NOT use SecureJoin,
// meaning that the passed string needs to be trusted. The caller _must_ call
// the returned procThreadSelfCloser function (which is runtime.UnlockOSThread)
// *only once* after it has finished using the returned path string.
func ProcThreadSelf(subpath string) (string, ProcThreadSelfCloser) {
	haveProcThreadSelfOnce.Do(func() {
		if _, err := os.Stat("/proc/thread-self/"); err == nil {
			haveProcThreadSelf = true
		} else {
			logrus.Debugf("cannot stat /proc/thread-self (%v), falling back to /proc/self/task/<tid>", err)
		}
	})

	// We need to lock our thread until the caller is done with the path string
	// because any non-atomic operation on the path (such as opening a file,
	// then reading it) could be interrupted by the Go runtime where the
	// underlying thread is swapped out and the original thread is killed,
	// resulting in pull-your-hair-out-hard-to-debug issues in the caller. In
	// addition, the pre-3.17 fallback makes everything non-atomic because the
	// same thing could happen between unix.Gettid() and the path operations.
	//
	// In theory, we don't need to lock in the atomic user case when using
	// /proc/thread-self/, but it's better to be safe than sorry (and there are
	// only one or two truly atomic users of /proc/thread-self/).
	runtime.LockOSThread()

	threadSelf := "/proc/thread-self/"
	if !haveProcThreadSelf {
		// Pre-3.17 kernels did not have /proc/thread-self, so do it manually.
		threadSelf = "/proc/self/task/" + strconv.Itoa(unix.Gettid()) + "/"
		if _, err := os.Stat(threadSelf); err != nil {
			// Unfortunately, this code is called from rootfs_linux.go where we
			// are running inside the pid namespace of the container but /proc
			// is the host's procfs. Unfortunately there is no real way to get
			// the correct tid to use here (the kernel age means we cannot do
			// things like set up a private fsopen("proc") -- even scanning
			// NSpid in all of the tasks in /proc/self/task/*/status requires
			// Linux 4.1).
			//
			// So, we just have to assume that /proc/self is acceptable in this
			// one specific case.
			if os.Getpid() == 1 {
				logrus.Debugf("/proc/thread-self (tid=%d) cannot be emulated inside the initial container setup -- using /proc/self instead: %v", unix.Gettid(), err)
			} else {
				// This should never happen, but the fallback should work in most cases...
				logrus.Warnf("/proc/thread-self could not be emulated for pid=%d (tid=%d) -- using more buggy /proc/self fallback instead: %v", os.Getpid(), unix.Gettid(), err)
			}
			threadSelf = "/proc/self/"
		}
	}
	return threadSelf + subpath, runtime.UnlockOSThread
}

//go:build !windows
// +build !windows

package utils

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

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

// CloseExecFrom applies O_CLOEXEC to all file descriptors currently open for
// the process (except for those below the given fd value).
func CloseExecFrom(minFd int) error {
	fdDir, err := os.Open("/proc/self/fd")
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

// NewSockPair returns a new unix socket pair
func NewSockPair(name string) (parent *os.File, child *os.File, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_STREAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), name+"-p"), os.NewFile(uintptr(fds[0]), name+"-c"), nil
}

// RunWithUmask runs a function f with umask mask.
// It does so while locking the OS thread, as umask
// operates on threads, and this avoid potentially leaking umask(0000)
// to other threads.
func RunWithUmask(mask int, f func() error) error {
	errC := make(chan error)
	go func() {
		runtime.LockOSThread()
		// The umask is part of the "filesystem information" whose sharing is governed by the `CLONE_FS` flag.
		// The Go runtime clones new threads with the `CLONE_FS` flag set, so any call to umask immediately affects
		// all goroutines in the process.
		if err := unix.Unshare(unix.CLONE_FS); err != nil {
			errC <- err
			runtime.UnlockOSThread()
			return
		}
		// However, unshare(CLONE_FS) is irreversible. Thus, ensure the thread is terminated by
		// not calling UnlockOSThread (which signals to the Go runtime to terminate or wedge the thread),
		// so this OSThread is not reused in other parts of the program.
		unix.Umask(mask)
		errC <- f()
	}()
	return <-errC
}

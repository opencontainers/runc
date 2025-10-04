package linux

import (
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Dup3 wraps [unix.Dup3].
func Dup3(oldfd, newfd, flags int) error {
	err := retryOnEINTR(func() error {
		return unix.Dup3(oldfd, newfd, flags)
	})
	return os.NewSyscallError("dup3", err)
}

// Exec wraps [unix.Exec].
func Exec(cmd string, args []string, env []string) error {
	err := retryOnEINTR(func() error {
		return unix.Exec(cmd, args, env)
	})
	if err != nil {
		return &os.PathError{Op: "exec", Path: cmd, Err: err}
	}
	return nil
}

// Getwd wraps [unix.Getwd].
func Getwd() (wd string, err error) {
	wd, err = retryOnEINTR2(unix.Getwd)
	return wd, os.NewSyscallError("getwd", err)
}

// Open wraps [unix.Open].
func Open(path string, mode int, perm uint32) (fd int, err error) {
	fd, err = retryOnEINTR2(func() (int, error) {
		return unix.Open(path, mode, perm)
	})
	if err != nil {
		return -1, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return fd, nil
}

// Openat wraps [unix.Openat].
func Openat(dirfd int, path string, mode int, perm uint32) (fd int, err error) {
	fd, err = retryOnEINTR2(func() (int, error) {
		return unix.Openat(dirfd, path, mode, perm)
	})
	if err != nil {
		return -1, &os.PathError{Op: "openat", Path: path, Err: err}
	}
	return fd, nil
}

// Recvfrom wraps [unix.Recvfrom].
func Recvfrom(fd int, p []byte, flags int) (n int, from unix.Sockaddr, err error) {
	err = retryOnEINTR(func() error {
		n, from, err = unix.Recvfrom(fd, p, flags)
		return err
	})
	if err != nil {
		return 0, nil, os.NewSyscallError("recvfrom", err)
	}
	return n, from, err
}

// Sendmsg wraps [unix.Sendmsg].
func Sendmsg(fd int, p, oob []byte, to unix.Sockaddr, flags int) error {
	err := retryOnEINTR(func() error {
		return unix.Sendmsg(fd, p, oob, to, flags)
	})
	return os.NewSyscallError("sendmsg", err)
}

// SetMempolicy wraps set_mempolicy.
func SetMempolicy(mode uint, mask *unix.CPUSet) error {
	err := retryOnEINTR(func() error {
		_, _, errno := unix.Syscall(unix.SYS_SET_MEMPOLICY, uintptr(mode), uintptr(unsafe.Pointer(mask)), unsafe.Sizeof(*mask)*8)
		if errno != 0 {
			return errno
		}
		return nil
	})
	return os.NewSyscallError("set_mempolicy", err)
}

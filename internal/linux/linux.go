package linux

import (
	"os"

	"golang.org/x/sys/unix"
)

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

// Sendmsg wraps [unix.Sendmsg].
func Sendmsg(fd int, p, oob []byte, to unix.Sockaddr, flags int) error {
	err := retryOnEINTR(func() error {
		return unix.Sendmsg(fd, p, oob, to, flags)
	})
	return os.NewSyscallError("sendmsg", err)
}

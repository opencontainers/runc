package linux

import (
	"os"

	"golang.org/x/sys/unix"
)

// Readlinkat wraps [unix.Readlinkat].
func Readlinkat(dir *os.File, path string) (string, error) {
	size := 4096
	for {
		linkBuf := make([]byte, size)
		n, err := unix.Readlinkat(int(dir.Fd()), path, linkBuf)
		if err != nil {
			return "", &os.PathError{Op: "readlinkat", Path: dir.Name() + "/" + path, Err: err}
		}
		if n != size {
			return string(linkBuf[:n]), nil
		}
		// Possible truncation, resize the buffer.
		size *= 2
	}
}

// GetPtyPeer is a wrapper for ioctl(TIOCGPTPEER).
func GetPtyPeer(ptyFd uintptr, unsafePeerPath string, flags int) (*os.File, error) {
	// Make sure O_NOCTTY is always set -- otherwise runc might accidentally
	// gain it as a controlling terminal. O_CLOEXEC also needs to be set to
	// make sure we don't leak the handle either.
	flags |= unix.O_NOCTTY | unix.O_CLOEXEC

	// There is no nice wrapper for this kind of ioctl in unix.
	peerFd, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		ptyFd,
		uintptr(unix.TIOCGPTPEER),
		uintptr(flags),
	)
	if errno != 0 {
		return nil, os.NewSyscallError("ioctl TIOCGPTPEER", errno)
	}
	return os.NewFile(peerFd, unsafePeerPath), nil
}

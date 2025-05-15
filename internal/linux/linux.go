package linux

import (
	"os"

	"golang.org/x/sys/unix"
)

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

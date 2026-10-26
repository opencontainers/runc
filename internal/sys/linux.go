//go:build linux

package sys

import (
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

type ParentDeathSignal int

func (p ParentDeathSignal) Restore() error {
	if p == 0 {
		return nil
	}
	current, err := GetParentDeathSignal()
	if err != nil {
		return err
	}
	if p == current {
		return nil
	}
	return p.Set()
}

func (p ParentDeathSignal) Set() error {
	return SetParentDeathSignal(uintptr(p))
}

func SetParentDeathSignal(sig uintptr) error {
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, sig, 0, 0, 0); err != nil {
		return err
	}
	return nil
}

func GetParentDeathSignal() (ParentDeathSignal, error) {
	var sig int
	if err := unix.Prctl(unix.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0, 0, 0); err != nil {
		return -1, err
	}
	return ParentDeathSignal(sig), nil
}

func SetKeepCaps() error {
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 1, 0, 0, 0); err != nil {
		return err
	}

	return nil
}

func ClearKeepCaps() error {
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 0, 0, 0, 0); err != nil {
		return err
	}

	return nil
}

func Setctty() error {
	if err := unix.IoctlSetInt(0, unix.TIOCSCTTY, 0); err != nil {
		return err
	}
	return nil
}

// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(i), 0, 0, 0)
}

// GetSubreaper returns the subreaper setting for the calling process
func GetSubreaper() (int, error) {
	var i uintptr

	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}

	return int(i), nil
}

// Copy is like io.Copy except it uses sendfile(2) if the source and sink are
// both (*os.File) as an optimisation to make copies faster.
func Copy(dst io.Writer, src io.Reader) (copied int64, err error) {
	dstFile, _ := dst.(*os.File)
	srcFile, _ := src.(*os.File)

	if dstFile != nil && srcFile != nil {
		fi, err := srcFile.Stat()
		if err != nil {
			goto fallback
		}
		size := fi.Size()
		for size > 0 {
			n, err := unix.Sendfile(int(dstFile.Fd()), int(srcFile.Fd()), nil, int(size))
			if n > 0 {
				size -= int64(n)
				copied += int64(n)
			}
			if err == unix.EINTR {
				continue
			}
			if err != nil {
				if copied == 0 {
					// If we haven't copied anything so far, we can safely just
					// fallback to io.Copy. We could always do the fallback but
					// it's safer to error out in the case of a partial copy
					// followed by an error (which should never happen).
					goto fallback
				}
				return copied, fmt.Errorf("partial sendfile copy: %w", err)
			}
		}
		return copied, nil
	}

fallback:
	return io.Copy(dst, src)
}

// SetLinuxPersonality sets the Linux execution personality. For more information see the personality syscall documentation.
// checkout getLinuxPersonalityFromStr() from libcontainer/specconv/spec_linux.go for type conversion.
func SetLinuxPersonality(personality int) error {
	_, _, errno := unix.Syscall(unix.SYS_PERSONALITY, uintptr(personality), 0, 0)
	if errno != 0 {
		return &os.SyscallError{Syscall: "set_personality", Err: errno}
	}
	return nil
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

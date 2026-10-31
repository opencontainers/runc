//go:build linux

package system

import (
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/internal/sys"
)

// ParentDeathSignal is a parent death signal, as set by prctl(PR_SET_PDEATHSIG).
//
// Deprecated: This type is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
type ParentDeathSignal = sys.ParentDeathSignal

// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func SetParentDeathSignal(sig uintptr) error {
	return sys.SetParentDeathSignal(sig)
}

// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func GetParentDeathSignal() (ParentDeathSignal, error) {
	return sys.GetParentDeathSignal()
}

// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func SetKeepCaps() error {
	return linux.SetKeepCaps()
}

// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func ClearKeepCaps() error {
	return linux.ClearKeepCaps()
}

// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func Setctty() error {
	return linux.Setctty()
}

// SetSubreaper sets the value i as the subreaper setting for the calling process
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func SetSubreaper(i int) error {
	return linux.SetSubreaper(i)
}

// GetSubreaper returns the subreaper setting for the calling process
//
// Deprecated: This function is deprecated and will be removed in runc 1.7.
func GetSubreaper() (int, error) {
	var i uintptr

	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}

	return int(i), nil
}

// Copy is like io.Copy except it uses sendfile(2) if the source and sink are
// both (*os.File) as an optimisation to make copies faster.
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func Copy(dst io.Writer, src io.Reader) (copied int64, err error) {
	return sys.Copy(dst, src)
}

// SetLinuxPersonality sets the Linux execution personality. For more information see the personality syscall documentation.
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func SetLinuxPersonality(personality int) error {
	return linux.SetLinuxPersonality(personality)
}

// GetPtyPeer is a wrapper for ioctl(TIOCGPTPEER).
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func GetPtyPeer(ptyFd uintptr, unsafePeerPath string, flags int) (*os.File, error) {
	return linux.GetPtyPeer(ptyFd, unsafePeerPath, flags)
}

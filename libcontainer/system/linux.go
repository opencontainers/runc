//go:build linux
// +build linux

package system

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"unsafe"

	"github.com/sirupsen/logrus"
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

func Execv(cmd string, args []string, env []string) error {
	name, err := exec.LookPath(cmd)
	if err != nil {
		return err
	}

	return Exec(name, args, env)
}

func Exec(cmd string, args []string, env []string) error {
	for {
		err := unix.Exec(cmd, args, env)
		if err != unix.EINTR { //nolint:errorlint // unix errors are bare
			return &os.PathError{Op: "exec", Path: cmd, Err: err}
		}
	}
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

func ExecutableMemfd(comment string, flags int) (*os.File, error) {
	// Try to use MFD_EXEC first. On pre-6.3 kernels we get -EINVAL for this
	// flag. On post-6.3 kernels, with vm.memfd_noexec=1 this ensures we get an
	// executable memfd. For vm.memfd_noexec=2 this is a bit more complicated.
	// The original vm.memfd_noexec=2 implementation incorrectly silently
	// allowed MFD_EXEC[1] -- this should be fixed in 6.6. On 6.6 and newer
	// kernels, we will get -EACCES if we try to use MFD_EXEC with
	// vm.memfd_noexec=2 (for 6.3-6.5, -EINVAL was the intended return value).
	//
	// The upshot is we only need to retry without MFD_EXEC on -EINVAL because
	// it just so happens that passing MFD_EXEC bypasses vm.memfd_noexec=2 on
	// kernels where -EINVAL is actually a security denial.
	memfd, err := unix.MemfdCreate(comment, flags|unix.MFD_EXEC)
	if errors.Is(err, unix.EINVAL) {
		memfd, err = unix.MemfdCreate(comment, flags)
	}
	if err != nil {
		if errors.Is(err, unix.EACCES) {
			logrus.Infof("memfd_create(MFD_EXEC) failed, possibly due to vm.memfd_noexec=2 -- falling back to less secure O_TMPFILE")
		}
		err := os.NewSyscallError("memfd_create", err)
		return nil, fmt.Errorf("failed to create executable memfd: %w", err)
	}
	return os.NewFile(uintptr(memfd), "/memfd:"+comment), nil
}

// Copy is a wrapper around io.Copy that continues the copy despite EINTR.
func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	for {
		n, err := io.Copy(dst, src)
		written += n
		if !errors.Is(err, unix.EINTR) {
			return written, err
		}
	}
}

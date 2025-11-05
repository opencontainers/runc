package sys

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/pathrs"
)

// FchmodFile is a wrapper around fchmodat2(AT_EMPTY_PATH) with fallbacks for
// older kernels. This is distinct from [File.Chmod] and [unix.Fchmod] in that
// it works on O_PATH file descriptors.
func FchmodFile(f *os.File, mode uint32) error {
	err := unix.Fchmodat(int(f.Fd()), "", mode, unix.AT_EMPTY_PATH)
	// If fchmodat2(2) is not available at all, golang.org/x/unix (probably
	// in order to mirror glibc) returns EOPNOTSUPP rather than EINVAL
	// (what the kernel actually returns for invalid flags, which is being
	// emulated) or ENOSYS (which is what glibc actually sees).
	if err != unix.EINVAL && err != unix.EOPNOTSUPP { //nolint:errorlint // unix errors are bare
		// err == nil is implicitly handled
		return os.NewSyscallError("fchmodat2 AT_EMPTY_PATH", err)
	}

	// AT_EMPTY_PATH support was added to fchmodat2 in Linux 6.6
	// (5daeb41a6fc9d0d81cb2291884b7410e062d8fa1). The alternative for
	// older kernels is to go through /proc.
	fdDir, closer, err2 := pathrs.ProcThreadSelfOpen("fd/", unix.O_DIRECTORY)
	if err2 != nil {
		return fmt.Errorf("fchmodat2 AT_EMPTY_PATH fallback: %w", err2)
	}
	defer closer()
	defer fdDir.Close()

	err = unix.Fchmodat(int(fdDir.Fd()), strconv.Itoa(int(f.Fd())), mode, 0)
	if err != nil {
		err = fmt.Errorf("fchmodat /proc/self/fd/%d: %w", f.Fd(), err)
	}
	runtime.KeepAlive(f)
	return err
}

// FchownFile is a wrapper around fchownat(AT_EMPTY_PATH). This is distinct
// from [File.Chown] and [unix.Fchown] in that it works on O_PATH file
// descriptors.
func FchownFile(f *os.File, uid, gid int) error {
	err := unix.Fchownat(int(f.Fd()), "", uid, gid, unix.AT_EMPTY_PATH)
	runtime.KeepAlive(f)
	return os.NewSyscallError("fchownat AT_EMPTY_PATH", err)
}

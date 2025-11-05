package sys

import (
	"fmt"
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

// VerifyInodeFunc is the callback passed to [VerifyInode] to check if the
// inode is the expected type (and on the correct filesystem type, in the case
// of filesystem-specific inodes).
type VerifyInodeFunc func(stat *unix.Stat_t, statfs *unix.Statfs_t) error

// VerifyInode verifies that the underlying inode for the given file matches an
// expected inode type (possibly on a particular kind of filesystem). This is
// mainly a wrapper around [VerifyInodeFunc].
func VerifyInode(file *os.File, checkFunc VerifyInodeFunc) error {
	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return fmt.Errorf("fstat %q: %w", file.Name(), err)
	}
	var statfs unix.Statfs_t
	if err := unix.Fstatfs(int(file.Fd()), &statfs); err != nil {
		return fmt.Errorf("fstatfs %q: %w", file.Name(), err)
	}
	runtime.KeepAlive(file)
	return checkFunc(&stat, &statfs)
}

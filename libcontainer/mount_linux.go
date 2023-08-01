package libcontainer

import (
	"errors"
	"io/fs"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// mountError holds an error from a failed mount or unmount operation.
type mountError struct {
	op     string
	source string
	srcFD  *int
	target string
	dstFD  string
	flags  uintptr
	data   string
	err    error
}

// Error provides a string error representation.
func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += "src=" + e.source + ", "
		if e.srcFD != nil {
			out += "srcFD=" + strconv.Itoa(*e.srcFD) + ", "
		}
	}
	out += "dst=" + e.target
	if e.dstFD != "" {
		out += ", dstFD=" + e.dstFD
	}

	if e.flags != uintptr(0) {
		out += ", flags=0x" + strconv.FormatUint(uint64(e.flags), 16)
	}
	if e.data != "" {
		out += ", data=" + e.data
	}

	out += ": " + e.err.Error()
	return out
}

// Unwrap returns the underlying error.
// This is a convention used by Go 1.13+ standard library.
func (e *mountError) Unwrap() error {
	return e.err
}

// mount is a simple unix.Mount wrapper, returning an error with more context
// in case it failed.
func mount(source, target, fstype string, flags uintptr, data string) error {
	return mountViaFDs(source, nil, target, "", fstype, flags, data)
}

// mountViaFDs is a unix.Mount wrapper which uses srcFD instead of source,
// and dstFD instead of target, unless those are empty.
//
// If srcFD is non-nil and flags contains MS_BIND, mountViaFDs will attempt to
// do a move_mount(2) as srcFD is assumed to be a "new mount API" file
// descriptor that cannot always be mounted using the old mount API (such as
// with bind-mounts).
//
// The dstFD argument, if non-empty, is expected to be in the form of a path to
// an opened file descriptor on procfs (i.e. "/proc/self/fd/NN").
//
// If case an FD is used instead of a source or a target path, the
// corresponding path is only used to add context to an error in case
// the mount operation has failed.
func mountViaFDs(source string, srcFD *int, target, dstFD, fstype string, flags uintptr, data string) error {
	dst := target
	if dstFD != "" {
		dst = dstFD
	}
	src := source
	if srcFD != nil {
		src = "/proc/self/fd/" + strconv.Itoa(*srcFD)
	}

	// Do a move_mount(2) if the source is a mountfd. For MS_REMOUNT,
	// move_mount(2) doesn't make sense (though we should never get into that
	// case).
	if srcFD != nil && flags&unix.MS_REMOUNT == 0 {
		if err := unix.MoveMount(*srcFD, "", unix.AT_FDCWD, dstFD, unix.MOVE_MOUNT_F_EMPTY_PATH|unix.MOVE_MOUNT_T_SYMLINKS); err == nil {
			// all done
			return nil
		} else if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOSYS) {
			// fall back to old-school mount -- even if it's almost certainly
			// not going to work...
			logrus.Debugf("move_mount of srcfd-based mount failed, falling back to classic mount")
		} else {
			// Prettify the source argument so the user knows what
			// /proc/self/fd/... refers to here.
			srcLink, err := os.Readlink(src)
			if err != nil {
				srcLink = "<unknown source path>"
			}
			return &mountError{
				op:     "move_mount",
				source: srcLink,
				srcFD:  srcFD,
				target: target,
				dstFD:  dstFD,
				flags:  flags,
				data:   data,
				err:    err,
			}
		}
	}
	if err := unix.Mount(src, dst, fstype, flags, data); err != nil {
		return &mountError{
			op:     "mount",
			source: source,
			srcFD:  srcFD,
			target: target,
			dstFD:  dstFD,
			flags:  flags,
			data:   data,
			err:    err,
		}
	}
	return nil
}

// unmount is a simple unix.Unmount wrapper.
func unmount(target string, flags int) error {
	err := unix.Unmount(target, flags)
	if err != nil {
		return &mountError{
			op:     "unmount",
			target: target,
			flags:  uintptr(flags),
			err:    err,
		}
	}
	return nil
}

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
// Copy from https://cs.opensource.google/go/go/+/refs/tags/go1.20.7:src/os/file_posix.go;l=61-75
func syscallMode(i fs.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&fs.ModeSetuid != 0 {
		o |= unix.S_ISUID
	}
	if i&fs.ModeSetgid != 0 {
		o |= unix.S_ISGID
	}
	if i&fs.ModeSticky != 0 {
		o |= unix.S_ISVTX
	}
	// No mapping for Go's ModeTemporary (plan9 only).
	return
}

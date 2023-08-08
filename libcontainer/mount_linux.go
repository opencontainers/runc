package libcontainer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/userns"
)

// mountSourceType indicates what type of file descriptor is being returned. It
// is used to tell rootfs_linux.go whether or not to use move_mount(2) to
// install the mount.
type mountSourceType string

const (
	// An open_tree(2)-style file descriptor that needs to be installed using
	// move_mount(2) to install.
	mountSourceOpenTree mountSourceType = "open_tree"
	// A plain file descriptor that can be mounted through /proc/self/fd.
	mountSourcePlain mountSourceType = "plain-open"
)

type mountSource struct {
	Type mountSourceType `json:"source_type"`
	file *os.File        `json:"-"`
}

// mountError holds an error from a failed mount or unmount operation.
type mountError struct {
	op      string
	source  string
	srcFile *mountSource
	target  string
	dstFd   string
	flags   uintptr
	data    string
	err     error
}

// Error provides a string error representation.
func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += "src=" + e.source + ", "
		if e.srcFile != nil {
			out += "srcType=" + string(e.srcFile.Type) + ", "
			out += "srcFd=" + strconv.Itoa(int(e.srcFile.file.Fd())) + ", "
		}
	}
	out += "dst=" + e.target
	if e.dstFd != "" {
		out += ", dstFd=" + e.dstFd
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
	return mountViaFds(source, nil, target, "", fstype, flags, data)
}

// mountViaFds is a unix.Mount wrapper which uses srcFile instead of source,
// and dstFd instead of target, unless those are empty.
//
// If srcFile is non-nil and flags does not contain MS_REMOUNT, mountViaFds
// will mount it according to the mountSourceType of the file descriptor.
//
// The dstFd argument, if non-empty, is expected to be in the form of a path to
// an opened file descriptor on procfs (i.e. "/proc/self/fd/NN").
//
// If a file descriptor is used instead of a source or a target path, the
// corresponding path is only used to add context to an error in case the mount
// operation has failed.
func mountViaFds(source string, srcFile *mountSource, target, dstFd, fstype string, flags uintptr, data string) error {
	// MS_REMOUNT and srcFile don't make sense together.
	if srcFile != nil && flags&unix.MS_REMOUNT != 0 {
		logrus.Debugf("mount source passed along with MS_REMOUNT -- ignoring srcFile")
		srcFile = nil
	}

	dst := target
	if dstFd != "" {
		dst = dstFd
	}
	src := source
	if srcFile != nil {
		src = "/proc/self/fd/" + strconv.Itoa(int(srcFile.file.Fd()))
	}

	var op string
	var err error
	if srcFile != nil && srcFile.Type == mountSourceOpenTree {
		op = "move_mount"
		err = unix.MoveMount(int(srcFile.file.Fd()), "",
			unix.AT_FDCWD, dstFd,
			unix.MOVE_MOUNT_F_EMPTY_PATH|unix.MOVE_MOUNT_T_SYMLINKS)
	} else {
		op = "mount"
		err = unix.Mount(src, dst, fstype, flags, data)
	}
	if err != nil {
		return &mountError{
			op:      op,
			source:  source,
			srcFile: srcFile,
			target:  target,
			dstFd:   dstFd,
			flags:   flags,
			data:    data,
			err:     err,
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

// mountFd creates an open_tree(2)-like mount fd from the provided
// configuration. This function must be called from within the container's
// mount namespace.
func mountFd(nsHandles *userns.Handles, m *configs.Mount) (mountSource, error) {
	if !m.IsBind() {
		return mountSource{}, errors.New("new mount api: only bind-mounts are supported")
	}
	if nsHandles == nil {
		nsHandles = new(userns.Handles)
		defer nsHandles.Release()
	}

	var mountFile *os.File
	var sourceType mountSourceType

	if m.IsBind() {
		// We only need to use OPEN_TREE_CLONE in the case where we need to use
		// mount_setattr(2). We are currently in the container namespace and
		// there is no risk of an opened directory being used to escape the
		// container. OPEN_TREE_CLONE is more expensive than open(2) because it
		// requires doing mounts inside a new anonymous mount namespace.
		if m.IsIDMapped() {
			flags := uint(unix.OPEN_TREE_CLONE | unix.O_CLOEXEC)
			if m.Flags&unix.MS_REC == unix.MS_REC {
				flags |= unix.AT_RECURSIVE
			}
			mountFd, err := unix.OpenTree(unix.AT_FDCWD, m.Source, flags)
			if err != nil {
				return mountSource{}, &os.PathError{Op: "open_tree(OPEN_TREE_CLONE)", Path: m.Source, Err: err}
			}
			mountFile = os.NewFile(uintptr(mountFd), m.Source)
			sourceType = mountSourceOpenTree
		} else {
			var err error
			mountFile, err = os.OpenFile(m.Source, unix.O_PATH|unix.O_CLOEXEC, 0)
			if err != nil {
				return mountSource{}, err
			}
			sourceType = mountSourcePlain
		}
	}

	if m.IsIDMapped() {
		if mountFile == nil {
			return mountSource{}, fmt.Errorf("invalid mount source %q: id-mapping of non-bind-mounts is not supported", m.Source)
		}
		if sourceType != mountSourceOpenTree {
			// should never happen
			return mountSource{}, fmt.Errorf("invalid mount source %q: id-mapped target mistakenly opened without OPEN_TREE_CLONE", m.Source)
		}

		usernsFile, err := nsHandles.Get(userns.Mapping{
			UIDMappings: m.UIDMappings,
			GIDMappings: m.GIDMappings,
		})
		if err != nil {
			return mountSource{}, fmt.Errorf("failed to create userns for %s id-mapping: %w", m.Source, err)
		}
		defer usernsFile.Close()
		if err := unix.MountSetattr(int(mountFile.Fd()), "", unix.AT_EMPTY_PATH, &unix.MountAttr{
			Attr_set:  unix.MOUNT_ATTR_IDMAP,
			Userns_fd: uint64(usernsFile.Fd()),
		}); err != nil {
			return mountSource{}, fmt.Errorf("failed to set IDMAP_SOURCE_ATTR on %s: %w", m.Source, err)
		}
	}
	return mountSource{
		Type: sourceType,
		file: mountFile,
	}, nil
}

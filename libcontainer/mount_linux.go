package libcontainer

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/internal/userns"
	"github.com/opencontainers/runc/libcontainer/system/kernelversion"
	"github.com/opencontainers/runc/libcontainer/utils"
)

// mountSourceType indicates what type of file descriptor is being returned. It
// is used to tell rootfs_linux.go whether or not to use move_mount(2) to
// install the mount.
type mountSourceType string

const (
	// An open_tree(2)-style file descriptor that needs to be installed using
	// move_mount(2) to install.
	mountSourceOpenTree mountSourceType = "open_tree"
	// A plain file descriptor that can be mounted through /proc/thread-self/fd.
	mountSourcePlain mountSourceType = "plain-open"
	// A file descriptor returned by fsopen(2) that needs to be installed using
	// fsmount(2) and move_mount(2).
	mountSourceFsOpen mountSourceType = "fsopen"
)

type mountSource struct {
	Type mountSourceType `json:"type"`
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

// int32plus is a collection of int types with >=32 bits.
type int32plus interface {
	int | uint | int32 | uint32 | int64 | uint64 | uintptr
}

// stringifyMountFlags converts mount(2) flags to a string that you can use in
// error messages.
func stringifyMountFlags[Int int32plus](flags Int) string {
	flagNames := []struct {
		name string
		bits Int
	}{
		{"MS_RDONLY", unix.MS_RDONLY},
		{"MS_NOSUID", unix.MS_NOSUID},
		{"MS_NODEV", unix.MS_NODEV},
		{"MS_NOEXEC", unix.MS_NOEXEC},
		{"MS_SYNCHRONOUS", unix.MS_SYNCHRONOUS},
		{"MS_REMOUNT", unix.MS_REMOUNT},
		{"MS_MANDLOCK", unix.MS_MANDLOCK},
		{"MS_DIRSYNC", unix.MS_DIRSYNC},
		{"MS_NOSYMFOLLOW", unix.MS_NOSYMFOLLOW},
		// No (1 << 9) flag.
		{"MS_NOATIME", unix.MS_NOATIME},
		{"MS_NODIRATIME", unix.MS_NODIRATIME},
		{"MS_BIND", unix.MS_BIND},
		{"MS_MOVE", unix.MS_MOVE},
		{"MS_REC", unix.MS_REC},
		// MS_VERBOSE was deprecated and swapped to MS_SILENT.
		{"MS_SILENT", unix.MS_SILENT},
		{"MS_POSIXACL", unix.MS_POSIXACL},
		{"MS_UNBINDABLE", unix.MS_UNBINDABLE},
		{"MS_PRIVATE", unix.MS_PRIVATE},
		{"MS_SLAVE", unix.MS_SLAVE},
		{"MS_SHARED", unix.MS_SHARED},
		{"MS_RELATIME", unix.MS_RELATIME},
		// MS_KERNMOUNT (1 << 22) is internal to the kernel.
		{"MS_I_VERSION", unix.MS_I_VERSION},
		{"MS_STRICTATIME", unix.MS_STRICTATIME},
		{"MS_LAZYTIME", unix.MS_LAZYTIME},
	}
	var (
		flagSet  []string
		seenBits Int
	)
	for _, flag := range flagNames {
		if flags&flag.bits == flag.bits {
			seenBits |= flag.bits
			flagSet = append(flagSet, flag.name)
		}
	}
	// If there were any remaining flags specified we don't know the name of,
	// just add them in an 0x... format.
	if remaining := flags &^ seenBits; remaining != 0 {
		flagSet = append(flagSet, "0x"+strconv.FormatUint(uint64(remaining), 16))
	}
	return strings.Join(flagSet, "|")
}

// Error provides a string error representation.
func (e *mountError) Error() string {
	out := e.op + " "

	if e.source != "" {
		out += "src=" + e.source + ", "
		if e.srcFile != nil && e.srcFile.file != nil {
			out += "srcType=" + string(e.srcFile.Type) + ", "
			out += "srcFd=" + strconv.Itoa(int(e.srcFile.file.Fd())) + ", "
		}
	}
	out += "dst=" + e.target
	if e.dstFd != "" {
		out += ", dstFd=" + e.dstFd
	}

	if e.flags != uintptr(0) {
		out += ", flags=" + stringifyMountFlags(e.flags)
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
// an opened file descriptor on procfs (i.e. "/proc/thread-self/fd/NN").
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
	isMoveMount := srcFile != nil && (srcFile.Type == mountSourceOpenTree || srcFile.Type == mountSourceFsOpen)
	if srcFile != nil {
		// If we're going to use the /proc/thread-self/... path for classic
		// mount(2), we need to get a safe handle to /proc/thread-self. This
		// isn't needed for move_mount(2) because in that case the path is just
		// a dummy string used for error info.
		srcFileFd := srcFile.file.Fd()
		if isMoveMount {
			src = "/proc/self/fd/" + strconv.Itoa(int(srcFileFd))
		} else {
			var closer utils.ProcThreadSelfCloser
			src, closer = utils.ProcThreadSelfFd(srcFileFd)
			defer closer()
		}
	}

	var op string
	var err error
	if isMoveMount {
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
	return o
}

// mountFd creates a "mount source fd" (either through open_tree(2) or just
// open(O_PATH)) based on the provided configuration. This function must be
// called from within the container's mount namespace.
//
// In the case of idmapped mount configurations, the returned mount source will
// be an open_tree(2) file with MOUNT_ATTR_IDMAP applied. For other
// bind-mounts, it will be an O_PATH. If the type of mount cannot be handled,
// the returned mountSource will be nil, indicating that the container init
// process will need to do an old-fashioned mount(2) themselves.
//
// This helper is only intended to be used by goCreateMountSources.
func mountFd(nsHandles *userns.Handles, m *configs.Mount) (_ *mountSource, retErr error) {
	if !m.IsBind() {
		return nil, errors.New("new mount api: only bind-mounts are supported")
	}
	if nsHandles == nil {
		nsHandles = new(userns.Handles)
		defer nsHandles.Release()
	}

	var mountFile *os.File
	var sourceType mountSourceType
	defer func() {
		if retErr != nil && mountFile != nil {
			mountFile.Close()
		}
	}()

	// Ideally, we would use OPEN_TREE_CLONE for everything, because we can
	// be sure that the file descriptor cannot be used to escape outside of
	// the mount root. Unfortunately, OPEN_TREE_CLONE is far more expensive
	// than open(2) because it requires doing mounts inside a new anonymous
	// mount namespace. So we use open(2) for standard bind-mounts, and
	// OPEN_TREE_CLONE when we need to set mount attributes here.
	//
	// While passing open(2)'d paths from the host rootfs isn't exactly the
	// safest thing in the world, the files will not survive across
	// execve(2) and "runc init" is non-dumpable so it should not be
	// possible for a malicious container process to gain access to the
	// file descriptors. We also don't do any of this for "runc exec",
	// lessening the risk even further.
	if m.IsIDMapped() {
		flags := uint(unix.OPEN_TREE_CLONE | unix.OPEN_TREE_CLOEXEC)
		if m.Flags&unix.MS_REC == unix.MS_REC {
			flags |= unix.AT_RECURSIVE
		}
		fd, err := unix.OpenTree(unix.AT_FDCWD, m.Source, flags)
		if err != nil {
			return nil, &os.PathError{Op: "open_tree(OPEN_TREE_CLONE)", Path: m.Source, Err: err}
		}
		mountFile = os.NewFile(uintptr(fd), m.Source)
		sourceType = mountSourceOpenTree

		// Configure the id mapping.
		var usernsFile *os.File
		if m.IDMapping.UserNSPath == "" {
			usernsFile, err = nsHandles.Get(userns.Mapping{
				UIDMappings: m.IDMapping.UIDMappings,
				GIDMappings: m.IDMapping.GIDMappings,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to create userns for %s id-mapping: %w", m.Source, err)
			}
		} else {
			usernsFile, err = os.Open(m.IDMapping.UserNSPath)
			if err != nil {
				return nil, fmt.Errorf("failed to open existing userns for %s id-mapping: %w", m.Source, err)
			}
		}
		defer usernsFile.Close()

		setAttrFlags := uint(unix.AT_EMPTY_PATH)
		// If the mount has "ridmap" set, we apply the configuration
		// recursively. This allows you to create "rbind" mounts where only
		// the top-level mount has an idmapping. I'm not sure why you'd
		// want that, but still...
		if m.IDMapping.Recursive {
			setAttrFlags |= unix.AT_RECURSIVE
		}
		if err := unix.MountSetattr(int(mountFile.Fd()), "", setAttrFlags, &unix.MountAttr{
			Attr_set:  unix.MOUNT_ATTR_IDMAP,
			Userns_fd: uint64(usernsFile.Fd()),
		}); err != nil {
			extraMsg := ""
			if err == unix.EINVAL {
				extraMsg = " (maybe the filesystem used doesn't support idmap mounts on this kernel?)"
			}

			return nil, fmt.Errorf("failed to set MOUNT_ATTR_IDMAP on %s: %w%s", m.Source, err, extraMsg)
		}
	} else {
		var err error
		mountFile, err = os.OpenFile(m.Source, unix.O_PATH|unix.O_CLOEXEC, 0)
		if err != nil {
			return nil, err
		}
		sourceType = mountSourcePlain
	}
	return &mountSource{
		Type: sourceType,
		file: mountFile,
	}, nil
}

// hasNewMountAPI reports whether the running kernel supports the new mount API
// (open_tree(2), move_mount(2), fsopen(2), fsmount(2)), which was introduced
// in Linux 5.2.
var hasNewMountAPI = sync.OnceValue(func() bool {
	// All of the pieces of the new mount API we use (fsopen, fsconfig,
	// fsmount, open_tree) were added together in Linux 5.2[1,2], so we can
	// just check for one of the syscalls and the others should also be
	// available.
	//
	// Just try to use open_tree(2) to open a file without OPEN_TREE_CLONE.
	// This is equivalent to openat(2), but tells us if open_tree is
	// available (and thus all of the other basic new mount API syscalls).
	// open_tree(2) is most light-weight syscall to test here.
	//
	// [1]: merge commit 400913252d09
	// [2]: <https://lore.kernel.org/lkml/153754740781.17872.7869536526927736855.stgit@warthog.procyon.org.uk/>
	fd, err := unix.OpenTree(-int(unix.EBADF), "/", unix.OPEN_TREE_CLOEXEC)
	if err != nil {
		return false
	}
	_ = unix.Close(fd)

	// RHEL 8 has a backport of fsopen(2) that appears to have some very
	// difficult to debug performance pathology. As such, it seems prudent to
	// simply reject pre-5.2 kernels.
	isNotBackport, _ := kernelversion.GreaterEqualThan(kernelversion.KernelVersion{Kernel: 5, Major: 2})
	return isNotBackport
})

// createDetachedBindMount creates a detached bind mount using open_tree(2).
func createDetachedBindMount(m *configs.Mount) (_ *mountSource, retErr error) {
	openTreeFlags := uint(unix.OPEN_TREE_CLONE | unix.OPEN_TREE_CLOEXEC)
	if m.Flags&unix.MS_REC != 0 {
		openTreeFlags |= unix.AT_RECURSIVE
	}
	fd, err := unix.OpenTree(unix.AT_FDCWD, m.Source, openTreeFlags)
	if err != nil {
		return nil, &os.PathError{Op: "open_tree", Path: m.Source, Err: err}
	}
	return &mountSource{
		Type: mountSourceOpenTree,
		file: os.NewFile(uintptr(fd), m.Source),
	}, nil
}

// fsconfigApplyOptions applies the options specified in opts to the filesystem
// file descriptor fsfd using fsconfig(2). The options are expected to be a
// comma-separated list of key=value pairs or flags.
func fsconfigApplyOptions(fsfd int, opts string) error {
	fields := strings.SplitSeq(opts, ",")
	for opt := range fields {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		k, v, found := strings.Cut(opt, "=")
		if found {
			if err := unix.FsconfigSetString(fsfd, k, v); err != nil {
				return fmt.Errorf("failed to set fsconfig option %s=%s: %w", k, v, err)
			}
		} else {
			if err := unix.FsconfigSetFlag(fsfd, opt); err != nil {
				return fmt.Errorf("failed to set fsconfig flag %s: %w", opt, err)
			}
		}
	}
	return nil
}

// msFlagsToMountAttr converts MS_* mount flags (as used by mount(2)) to
// MOUNT_ATTR_* flags (as used by fsmount(2) and mount_setattr(2)).
func msFlagsToMountAttr(flags int) int {
	attr := 0
	if flags&unix.MS_RDONLY != 0 {
		attr |= unix.MOUNT_ATTR_RDONLY
	}
	if flags&unix.MS_NOSUID != 0 {
		attr |= unix.MOUNT_ATTR_NOSUID
	}
	if flags&unix.MS_NODEV != 0 {
		attr |= unix.MOUNT_ATTR_NODEV
	}
	if flags&unix.MS_NOEXEC != 0 {
		attr |= unix.MOUNT_ATTR_NOEXEC
	}
	if flags&unix.MS_NOSYMFOLLOW != 0 {
		attr |= unix.MOUNT_ATTR_NOSYMFOLLOW
	}
	if flags&unix.MS_NODIRATIME != 0 {
		attr |= unix.MOUNT_ATTR_NODIRATIME
	}
	// The atime mode flags are mutually exclusive and map to a multi-bit
	// field (bits 4-6) in MOUNT_ATTR. MS_RELATIME maps to 0 (default).
	switch flags & (unix.MS_NOATIME | unix.MS_RELATIME | unix.MS_STRICTATIME) {
	case unix.MS_NOATIME:
		attr |= unix.MOUNT_ATTR_NOATIME
	case unix.MS_STRICTATIME:
		attr |= unix.MOUNT_ATTR_STRICTATIME
	}
	if flags&unix.MS_LAZYTIME != 0 {
		logrus.Warnf("MS_LAZYTIME mount flag specified, but kernel does not support MOUNT_ATTR_LAZYTIME")
	}
	return attr
}

// createDetachedFsMount creates a detached filesystem mount using fsopen(2),
// fsconfig(2), fsmount(2), and move_mount(2).
func createDetachedFsMount(m *configs.Mount, flagsMask int, mountLabel string) (_ *mountSource, retErr error) {
	fstype := m.Device
	if fstype == "cgroup" {
		fstype = "cgroup2"
	}
	fsfd, err := unix.Fsopen(fstype, unix.FSOPEN_CLOEXEC)
	if err != nil {
		return nil, &os.PathError{Op: "fsopen", Path: fstype, Err: err}
	}
	defer unix.Close(fsfd)

	if err := fsconfigApplyOptions(fsfd, m.Data); err != nil {
		return nil, &os.PathError{Op: "fsconfigset", Path: fstype, Err: err}
	}
	if mountLabel != "" {
		if err := unix.FsconfigSetString(fsfd, "context", mountLabel); err != nil {
			return nil, &os.PathError{Op: "fsconfigset(context)", Path: fstype, Err: err}
		}
	}
	source := m.Source
	if m.Device == "cgroup" {
		source = fstype
	}
	if err := unix.FsconfigSetString(fsfd, "source", source); err != nil {
		return nil, &os.PathError{Op: "fsconfigset(source)", Path: fstype, Err: err}
	}
	if err := unix.FsconfigCreate(fsfd); err != nil {
		return nil, &os.PathError{Op: "fsconfig", Path: fstype, Err: err}
	}

	flags := m.Flags
	if flagsMask != 0 {
		flags &= flagsMask
	}
	fd, err := unix.Fsmount(fsfd, unix.FSMOUNT_CLOEXEC, msFlagsToMountAttr(flags))
	if err != nil {
		return nil, &os.PathError{Op: "fsmount", Path: fstype, Err: err}
	}
	return &mountSource{
		Type: mountSourceFsOpen,
		file: os.NewFile(uintptr(fd), fstype),
	}, nil
}

// createDetachedMountSource creates a detached mount source for the given mount
// configuration. If the mount is a bind-mount, it will use open_tree(2). If it
// is a filesystem mount, it will use fsopen(2), fsconfig(2), and fsmount(2).
func createDetachedMountSource(m *configs.Mount, flagsMask int, mountLabel string) (_ *mountSource, retErr error) {
	if m.IsBind() {
		// 1. For mount flags:
		//    Since we create a detached bind mount, there is no need to set mount attributes upfront.
		//    They can be applied later using move_mount(2) and mount_setattr(2).
		//
		// 2. For mountLabel:
		//    Mount labels cannot be set on a detached bind mount, so we skip setting it here.
		//    As documented in mount(2): if MS_BIND is included in mountflags,
		//    the filesystem type and mount data arguments are ignored.
		return createDetachedBindMount(m)
	}
	return createDetachedFsMount(m, flagsMask, mountLabel)
}

package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
	"github.com/opencontainers/runc/internal/pathrs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/selinux/go-selinux/label"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// mountEntry contains mount data specific to a mount point.
type mountEntry struct {
	*configs.Mount
	srcFile *mountSource
	dstFile *os.File
}

// srcName is only meant for error messages, it returns a "friendly" name.
func (m mountEntry) srcName() string {
	if m.srcFile != nil {
		return m.srcFile.file.Name()
	}
	return m.Source
}

func (m mountEntry) srcStat() (os.FileInfo, *syscall.Stat_t, error) {
	var (
		st  os.FileInfo
		err error
	)
	if m.srcFile != nil {
		st, err = m.srcFile.file.Stat()
	} else {
		st, err = os.Stat(m.Source)
	}
	if err != nil {
		return nil, nil, err
	}
	return st, st.Sys().(*syscall.Stat_t), nil
}

func (m mountEntry) srcStatfs() (*unix.Statfs_t, error) {
	var st unix.Statfs_t
	if m.srcFile != nil {
		if err := unix.Fstatfs(int(m.srcFile.file.Fd()), &st); err != nil {
			return nil, os.NewSyscallError("fstatfs", err)
		}
	} else {
		if err := unix.Statfs(m.Source, &st); err != nil {
			return nil, &os.PathError{Op: "statfs", Path: m.Source, Err: err}
		}
	}
	return &st, nil
}

func (m mountEntry) isDetachedMount() bool {
	return m.srcFile != nil && (m.srcFile.Type == mountSourceOpenTree || m.srcFile.Type == mountSourceFsOpen)
}

func (m *mountEntry) createOpenMountpoint(root *os.File) (Err error) {
	rootfs := root.Name()
	unsafePath := pathrs.LexicallyStripRoot(rootfs, m.Destination)
	dstFile, err := pathrs.OpenInRoot(root, unsafePath, unix.O_PATH)
	defer func() {
		if dstFile != nil && Err != nil {
			_ = dstFile.Close()
		}
	}()
	if err == nil && m.Device == "tmpfs" {
		// If the original target exists, copy the mode for the tmpfs mount.
		stat, err := dstFile.Stat()
		if err != nil {
			return fmt.Errorf("check tmpfs source mode: %w", err)
		}
		dt := fmt.Sprintf("mode=%04o", syscallMode(stat.Mode()))
		if m.Data != "" {
			dt = dt + "," + m.Data
		}
		m.Data = dt
	}
	if err != nil {
		if !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("lookup mountpoint target: %w", err)
		}

		// If the mountpoint doesn't already exist, we want to create a mountpoint
		// that makes sense for the source. For file bind-mounts this is an empty
		// file, for everything else it's a directory.
		dstIsFile := false
		if m.Device == "bind" {
			fi, _, err := m.srcStat()
			if err != nil {
				// Error out if the source of a bind mount does not exist as we
				// will be unable to bind anything to it.
				return err
			}
			dstIsFile = !fi.IsDir()
		}
		if dstIsFile {
			dstFile, err = pathrs.CreateInRoot(root, unsafePath, unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW, 0o644)
		} else {
			dstFile, err = pathrs.MkdirAllInRoot(root, unsafePath, 0o755)
		}
		if err != nil {
			return fmt.Errorf("make mountpoint %q: %w", m.Destination, err)
		}
	}

	dstFullPath, err := procfs.ProcSelfFdReadlink(dstFile)
	if err != nil {
		return fmt.Errorf("get mount destination real path: %w", err)
	}
	if !pathrs.IsLexicallyInRoot(rootfs, dstFullPath) {
		return fmt.Errorf("mountpoint %q is outside of rootfs %q", dstFullPath, rootfs)
	}
	if relPath, err := filepath.Rel(rootfs, dstFullPath); err != nil {
		return fmt.Errorf("get relative path of %q: %w", dstFullPath, err)
	} else if relPath == "." {
		return fmt.Errorf("mountpoint %q is on the top of rootfs %q", dstFullPath, rootfs)
	}
	// TODO: Make checkProcMount use dstFile directly to avoid the need to
	// operate on paths here.
	if err := checkProcMount(rootfs, dstFullPath, *m); err != nil {
		return fmt.Errorf("check proc-safety of %s mount: %w", m.Destination, err)
	}
	// Update mountEntry.
	m.dstFile = dstFile
	return nil
}

func reopenAfterMount(rootFd, f *os.File, flags int) (_ *os.File, Err error) {
	fullPath, err := procfs.ProcSelfFdReadlink(f)
	if err != nil {
		return nil, fmt.Errorf("get full path: %w", err)
	}
	rootfs := rootFd.Name()
	if !pathrs.IsLexicallyInRoot(rootfs, fullPath) {
		return nil, fmt.Errorf("mountpoint %q is outside of rootfs %q", fullPath, rootfs)
	}
	unsafePath := pathrs.LexicallyStripRoot(rootfs, fullPath)
	reopened, err := pathrs.OpenInRoot(rootFd, unsafePath, flags)
	if err != nil {
		return nil, fmt.Errorf("re-open mountpoint %q: %w", unsafePath, err)
	}
	defer func() {
		if Err != nil {
			_ = reopened.Close()
		}
	}()

	// NOTE: The best we can do here is confirm that the new mountpoint handle
	// matches the original target handle, but an attacker could've swapped a
	// different path to replace it. In the worst case this could result in us
	// applying later vfsmount flags onto the wrong mount.
	//
	// This is far from ideal, but the only way of doing this in a race-free
	// way is to switch the new mount API (move_mount(2) does not require this
	// re-opening step, and thus no such races are possible).
	reopenedFullPath, err := procfs.ProcSelfFdReadlink(reopened)
	if err != nil {
		return nil, fmt.Errorf("check full path of re-opened mountpoint: %w", err)
	}
	if reopenedFullPath != fullPath {
		return nil, fmt.Errorf("mountpoint %q was moved while re-opening", unsafePath)
	}
	return reopened, nil
}

// initMountSourceFd initializes the mount source file descriptor if it is nil and the new mount API is available.
func (m *mountEntry) initMountSourceFd(flags int, mountLabel string) {
	if hasNewMountAPI() && (m.srcFile == nil || m.srcFile.file == nil) {
		if source, err := createDetachedMountSource(m.Mount, flags, mountLabel); err == nil {
			m.srcFile = source
		} else {
			logrus.Debugf("new mount api error: %v", err)
		}
	}
}

// Do the mount operation followed by additional mounts required to take care
// of propagation flags. This will always be scoped inside the container rootfs.
func (m *mountEntry) mountPropagate(rootFd *os.File, mountLabel string) error {
	var (
		data  = label.FormatMountLabel(m.Data, mountLabel)
		flags = m.Flags
	)
	// Delay mounting the filesystem read-only if we need to do further
	// operations on it. We need to set up files in "/dev", and other tmpfs
	// mounts may need to be chmod-ed after mounting. These mounts will be
	// remounted ro later in finalizeRootfs(), if necessary.
	flagsMask := 0
	if m.Device == "tmpfs" || pathrs.LexicallyCleanPath(m.Destination) == "/dev" {
		flagsMask = ^unix.MS_RDONLY
	}
	m.initMountSourceFd(flagsMask, mountLabel)

	if err := utils.WithProcfdFile(m.dstFile, func(dstFd string) error {
		if flagsMask != 0 {
			flags &= flagsMask
		}
		return mountViaFds(m.Source, m.srcFile, m.Destination, dstFd, m.Device, uintptr(flags), data)
	}); err != nil {
		return err
	}

	if m.isDetachedMount() {
		_ = m.dstFile.Close()
		m.dstFile = m.srcFile.file
	} else {
		// We need to re-open the mountpoint after doing the mount, in order for us
		// to operate on the new mount we just created. However, we cannot use
		// pathrs.Reopen because we need to re-resolve from the parent directory to
		// get a new handle to the top mount.
		newDstFile, err := reopenAfterMount(rootFd, m.dstFile, unix.O_PATH)
		if err != nil {
			return fmt.Errorf("reopen mountpoint after mount: %w", err)
		}
		_ = m.dstFile.Close()
		m.dstFile = newDstFile
	}

	// Apply the propagation flags on the new mount using mount_setattr(2) if
	// available, otherwise fall back to mount(2).
	if len(m.PropagationFlags) > 0 {
		// Try to use mount_setattr(2) for propagation flags. This is more
		// secure and efficient than using mount(2).
		//
		// PropagationFlags can contain MS_REC combined with propagation types
		// (e.g., "rprivate" = MS_PRIVATE | MS_REC). For mount_setattr, we need
		// to handle MS_REC separately as the AT_RECURSIVE flag parameter.
		var propagation uint64
		// Always use AT_EMPTY_PATH since we're operating on the fd directly
		pflags := uint(unix.AT_EMPTY_PATH)
		for _, pflag := range m.PropagationFlags {
			if pflag&unix.MS_REC != 0 {
				pflags |= unix.AT_RECURSIVE
			}
			// Only include the propagation type, not MS_REC
			propagation |= uint64(pflag & ^unix.MS_REC)
		}
		attr := unix.MountAttr{
			Propagation: propagation,
		}
		err := unix.MountSetattr(int(m.dstFile.Fd()), "", pflags, &attr)
		if err == nil {
			return nil
		}
		// fall back to mount(2).
		logrus.Debugf("mount_setattr failed for %s: %v, falling back to mount(2)", m.Destination, err)
		if err := utils.WithProcfdFile(m.dstFile, func(dstFd string) error {
			for _, pflag := range m.PropagationFlags {
				if err := mountViaFds("", nil, m.Destination, dstFd, "", uintptr(pflag), ""); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("change mount propagation through procfd: %w", err)
		}
	}
	return nil
}

func setRecAttr(m *mountEntry) error {
	if m.RecAttr == nil {
		return nil
	}
	return utils.WithProcfdFile(m.dstFile, func(procfd string) error {
		return unix.MountSetattr(-1, procfd, unix.AT_RECURSIVE, m.RecAttr)
	})
}

// cleanup closes any open file descriptors in the mountEntry. This is called
// after the mount has been completed and the mountEntry is no longer needed.
func (m *mountEntry) cleanup() {
	if m.srcFile != nil && m.srcFile.file != nil {
		if m.srcFile.file != m.dstFile {
			_ = m.srcFile.file.Close()
		}
		m.srcFile.file = nil
	}
	if m.dstFile != nil {
		_ = m.dstFile.Close()
		m.dstFile = nil
	}
}

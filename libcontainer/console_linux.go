package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/containerd/console"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/internal/pathrs"
	"github.com/opencontainers/runc/internal/sys"
	"github.com/opencontainers/runc/libcontainer/utils"
)

// checkPtmxHandle checks that the given file handle points to a real
// /dev/pts/ptmx device inode on a real devpts mount. We cannot (trivially)
// check that it is *the* /dev/pts for the container itself, but this is good
// enough.
func checkPtmxHandle(ptmx *os.File) error {
	//nolint:revive,staticcheck,nolintlint // ignore "don't use ALL_CAPS" warning // nolintlint is needed to work around the different lint configs
	const (
		PTMX_MAJOR = 5 // from TTYAUX_MAJOR in <linux/major.h>
		PTMX_MINOR = 2 // from mknod_ptmx in fs/devpts/inode.c
		PTMX_INO   = 2 // from mknod_ptmx in fs/devpts/inode.c
	)
	return sys.VerifyInode(ptmx, func(stat *unix.Stat_t, statfs *unix.Statfs_t) error {
		if statfs.Type != unix.DEVPTS_SUPER_MAGIC {
			return fmt.Errorf("ptmx handle is not on a real devpts mount: super magic is %#x", statfs.Type)
		}
		if stat.Ino != PTMX_INO {
			return fmt.Errorf("ptmx handle has wrong inode number: %v", stat.Ino)
		}
		rdev := uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on MIPS.
		if stat.Mode&unix.S_IFMT != unix.S_IFCHR || rdev != unix.Mkdev(PTMX_MAJOR, PTMX_MINOR) {
			return fmt.Errorf("ptmx handle is not a real char ptmx device: ftype %#x %d:%d",
				stat.Mode&unix.S_IFMT, unix.Major(rdev), unix.Minor(rdev))
		}
		return nil
	})
}

func isPtyNoIoctlError(err error) bool {
	// The kernel converts -ENOIOCTLCMD to -ENOTTY automatically, but handle
	// -EINVAL just in case (which some drivers do, include pty).
	return errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTTY)
}

func getPtyPeer(pty console.Console, unsafePeerPath string, flags int) (*os.File, error) {
	peer, err := linux.GetPtyPeer(pty.Fd(), unsafePeerPath, flags)
	if err == nil || !isPtyNoIoctlError(err) {
		return peer, err
	}

	// On pre-TIOCGPTPEER kernels (Linux < 4.13), we need to fallback to using
	// the /dev/pts/$n path generated using TIOCGPTN. We can do some validation
	// that the inode is correct because the Unix-98 pty has a consistent
	// numbering scheme for the device number of the peer.

	peerNum, err := unix.IoctlGetUint32(int(pty.Fd()), unix.TIOCGPTN)
	if err != nil {
		return nil, fmt.Errorf("get peer number of pty: %w", err)
	}
	//nolint:revive,staticcheck,nolintlint // ignore "don't use ALL_CAPS" warning // nolintlint is needed to work around the different lint configs
	const (
		UNIX98_PTY_SLAVE_MAJOR = 136 // from <linux/major.h>
	)
	wantPeerDev := unix.Mkdev(UNIX98_PTY_SLAVE_MAJOR, peerNum)

	// Use O_PATH to avoid opening a bad inode before we validate it.
	peerHandle, err := os.OpenFile(unsafePeerPath, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	defer peerHandle.Close()

	if err := sys.VerifyInode(peerHandle, func(stat *unix.Stat_t, statfs *unix.Statfs_t) error {
		if statfs.Type != unix.DEVPTS_SUPER_MAGIC {
			return fmt.Errorf("pty peer handle is not on a real devpts mount: super magic is %#x", statfs.Type)
		}
		rdev := uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on MIPS.
		if stat.Mode&unix.S_IFMT != unix.S_IFCHR || rdev != wantPeerDev {
			return fmt.Errorf("pty peer handle is not the real char device for pty %d: ftype %#x %d:%d",
				peerNum, stat.Mode&unix.S_IFMT, unix.Major(rdev), unix.Minor(rdev))
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return pathrs.Reopen(peerHandle, flags)
}

// safeAllocPty returns a new (ptmx, peer pty) allocation for use inside a
// container.
func safeAllocPty() (pty console.Console, peer *os.File, Err error) {
	// TODO: Use openat2(RESOLVE_NO_SYMLINKS|RESOLVE_NO_XDEV).
	ptmxHandle, err := os.OpenFile("/dev/pts/ptmx", unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	defer ptmxHandle.Close()

	if err := checkPtmxHandle(ptmxHandle); err != nil {
		return nil, nil, fmt.Errorf("verify ptmx handle: %w", err)
	}

	ptyFile, err := pathrs.Reopen(ptmxHandle, unix.O_RDWR|unix.O_NOCTTY)
	if err != nil {
		return nil, nil, fmt.Errorf("reopen ptmx to get new pty pair: %w", err)
	}
	// On success, the ownership is transferred to pty.
	defer func() {
		if Err != nil {
			_ = ptyFile.Close()
		}
	}()

	pty, unsafePeerPath, err := console.NewPtyFromFile(ptyFile)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if Err != nil {
			_ = pty.Close()
		}
	}()

	peer, err = getPtyPeer(pty, unsafePeerPath, unix.O_RDWR|unix.O_NOCTTY)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get peer end of newly-allocated console: %w", err)
	}
	return pty, peer, nil
}

// mountConsole bind-mounts the provided pty on top of /dev/console so programs
// that operate on /dev/console operate on the correct container pty.
func mountConsole(peerPty *os.File) error {
	console, err := os.OpenFile("/dev/console", unix.O_NOFOLLOW|unix.O_CREAT|unix.O_CLOEXEC, 0o666)
	if err != nil {
		return fmt.Errorf("create /dev/console mount target: %w", err)
	}
	defer console.Close()

	dstFd, closer := utils.ProcThreadSelfFd(console.Fd())
	defer closer()

	mntSrc := &mountSource{
		Type: mountSourcePlain,
		file: peerPty,
	}
	return mountViaFds(peerPty.Name(), mntSrc, "/dev/console", dstFd, "bind", unix.MS_BIND, "")
}

// dupStdio replaces stdio with the given peerPty.
func dupStdio(peerPty *os.File) error {
	for _, i := range []int{0, 1, 2} {
		if err := unix.Dup3(int(peerPty.Fd()), i, 0); err != nil {
			return err
		}
	}
	runtime.KeepAlive(peerPty)
	return nil
}

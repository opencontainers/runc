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
		if stat.Mode&unix.S_IFMT != unix.S_IFCHR || stat.Rdev != wantPeerDev {
			return fmt.Errorf("pty peer handle is not the real char device for pty %d: ftype %#x %d:%d",
				peerNum, stat.Mode&unix.S_IFMT, unix.Major(stat.Rdev), unix.Minor(stat.Rdev))
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
	pty, unsafePeerPath, err := console.NewPty()
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

package libcontainer

import (
	"fmt"
	"os"
	"runtime"

	"github.com/containerd/console"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/linux"
)

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

	peer, err = linux.GetPtyPeer(pty.Fd(), unsafePeerPath, unix.O_RDWR|unix.O_NOCTTY)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get peer end of newly-allocated console: %w", err)
	}
	return pty, peer, nil
}

// mountConsole bind-mounts the provided pty on top of /dev/console so programs
// that operate on /dev/console operate on the correct container pty.
func mountConsole(peerPty *os.File) error {
	f, err := os.Create("/dev/console")
	if err != nil && !os.IsExist(err) {
		return err
	}
	if f != nil {
		// Ensure permission bits (can be different because of umask).
		if err := f.Chmod(0o666); err != nil {
			return err
		}
		f.Close()
	}
	mntSrc := &mountSource{
		Type: mountSourcePlain,
		file: peerPty,
	}
	return mountViaFds(peerPty.Name(), mntSrc, "/dev/console", "", "bind", unix.MS_BIND, "")
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

package libcontainer

import (
	"os"

	unixutils "github.com/opencontainers/runc/libcontainer/internal/unix-utils"
	"golang.org/x/sys/unix"
)

// mount initializes the console inside the rootfs mounting with the specified mount label
// and applying the correct ownership of the console.
func mountConsole(slavePath string) error {
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
	return mount(slavePath, "/dev/console", "bind", unix.MS_BIND, "")
}

// dupStdio opens the slavePath for the console and dups the fds to the current
// processes stdio, fd 0,1,2.
func dupStdio(slavePath string) error {
	fd, err := unixutils.RetryOnEINTR2(func() (int, error) {
		return unix.Open(slavePath, unix.O_RDWR, 0)
	})
	if err != nil {
		return &os.PathError{
			Op:   "open",
			Path: slavePath,
			Err:  err,
		}
	}
	for _, i := range []int{0, 1, 2} {
		err := unixutils.RetryOnEINTR(func() error {
			return unix.Dup3(fd, i, 0)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

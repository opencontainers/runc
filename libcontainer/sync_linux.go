package libcontainer

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/system"
)

// waitForSyncReady blocks until either the sync socket is ready to read
// (data available, or cleanly closed) or the given process has exited.
// ready=false means the process is gone without its socket becoming ready;
// the caller should treat that the same as io.EOF rather than attempting a
// read, since a stray reference to the child's end of the socket elsewhere
// can otherwise leave a plain read blocked indefinitely.
func waitForSyncReady(f *os.File, pid int) (ready bool, err error) {
	pfd := []unix.PollFd{{Fd: int32(f.Fd()), Events: unix.POLLIN}}
	const pollIntervalMs = 100
	for {
		n, err := unix.Poll(pfd, pollIntervalMs)
		if errors.Is(err, unix.EINTR) {
			// Avoid a tight spin if signals are arriving rapidly (e.g. Go's
			// runtime async-preemption SIGURG).
			time.Sleep(time.Millisecond)
			continue
		}
		if err != nil {
			return false, fmt.Errorf("poll sync socket: %w", err)
		}
		if n > 0 && pfd[0].Revents&(unix.POLLIN|unix.POLLHUP) != 0 {
			return true, nil
		}
		stat, err := system.Stat(pid)
		if err != nil || stat.State == system.Zombie {
			return false, nil
		}
	}
}

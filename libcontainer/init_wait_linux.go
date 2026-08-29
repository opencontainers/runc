package libcontainer

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/system"
)

// initDeadError is returned when "runc init" died before saying what we were
// waiting for it to say.
type initDeadError struct {
	// startTime is the start time of the dead init, as read from
	// /proc/[pid]/stat when its death was noticed, or 0 if the process was
	// already gone by then. Use it to tell whether a pid still refers to
	// that very same process before acting on it.
	startTime uint64
}

func (*initDeadError) Error() string {
	return "container init process died unexpectedly"
}

// initPollTimeoutMs is how often waitForInitWrite rechecks that "runc init" is
// still alive.
const initPollTimeoutMs = 100

// initDead tells whether the "runc init" process with the given pid is dead,
// meaning it will never write anything to us again, returning a non-nil error
// if it is.
//
// A zombie counts as dead even though its thread group may still be alive.
// Seccomp's SCMP_ACT_KILL (SECCOMP_RET_KILL_THREAD) only kills the thread that
// performed the syscall, and "runc init" runs on the thread group leader (see
// [Init]). Once that thread is killed, the remaining Go runtime threads live
// on forever, keeping the shared descriptor table -- and thus their ends of
// the sync socket and the exec fifo -- open, so we never see an EOF. Neither
// wait(2) nor a pidfd report a zombie leader with live threads either, which
// leaves /proc/[pid]/stat as the only usable signal.
func initDead(pid int) *initDeadError {
	stat, err := system.Stat(pid)
	if err != nil {
		// Either the process is gone, or its /proc entry is unreadable;
		// either way, there is nothing left to wait for.
		return &initDeadError{}
	}
	if stat.State == system.Zombie || stat.State == system.Dead {
		return &initDeadError{startTime: stat.StartTime}
	}
	return nil
}

// waitForInitWrite blocks until the given fd, which "runc init" with the given
// pid is expected to write to, has something to read (or its writing end was
// closed). It returns an [initDeadError] if init dies before that happens.
//
// The pid can not be recycled while we are polling: init is either our own
// unreaped child, or a process whose disappearance is what we are looking for
// in the first place.
func waitForInitWrite(fd, pid int) error {
	pfds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		// Do not poll indefinitely -- a dead init is not always
		// detectable via the fd alone (see [initDead]).
		n, err := unix.Poll(pfds, initPollTimeoutMs)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return fmt.Errorf("poll init: %w", err)
		}
		// Any revents (including POLLHUP) means the caller can now read
		// without blocking, so let it -- data that init managed to write
		// before dying still has to be processed.
		if n > 0 {
			return nil
		}
		if err := initDead(pid); err != nil {
			return err
		}
	}
}

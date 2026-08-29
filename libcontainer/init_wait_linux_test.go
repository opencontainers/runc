package libcontainer

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const zombieLeaderEnv = "RUNC_TEST_ZOMBIE_LEADER"

// init turns this process into the helper for TestWaitForInitWriteZombieLeader
// when asked to: it kills its own thread group leader while the rest of its
// threads (and thus all of its file descriptors) stay alive, the way seccomp's
// SCMP_ACT_KILL does to "runc init".
//
// This has to happen from init, because that is the only place where the Go
// runtime guarantees we are running on the main thread (i.e. on the thread
// group leader).
func init() {
	if os.Getenv(zombieLeaderEnv) != "1" {
		return
	}

	// Keep a non-leader thread around forever. It has to block in a syscall
	// rather than on a channel, or the Go runtime declares a deadlock and
	// terminates the whole process.
	started := make(chan struct{})
	go func() {
		runtime.LockOSThread()
		close(started)
		for {
			_ = unix.Pause()
		}
	}()
	<-started

	if unix.Gettid() != unix.Getpid() {
		panic("zombie leader helper is not running on the thread group leader")
	}
	// Terminate this thread, and only this thread.
	_, _, _ = unix.RawSyscall(unix.SYS_EXIT, 0, 0, 0)
}

func TestWaitForInitWriteZombieLeader(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), zombieLeaderEnv+"=1")
	cmd.ExtraFiles = []*os.File{w}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	// Only the helper is supposed to hold the writing end now.
	w.Close()
	defer func() {
		// SIGKILL is group-wide, so it does what the helper's own
		// exit(2) could not.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	done := make(chan error, 1)
	go func() {
		done <- waitForInitWrite(int(r.Fd()), cmd.Process.Pid)
	}()

	select {
	case err := <-done:
		dead, ok := errors.AsType[*initDeadError](err)
		if !ok {
			t.Fatalf("waitForInitWrite: want *initDeadError, got %v", err)
		}
		if dead.startTime == 0 {
			t.Error("waitForInitWrite: no start time in the error")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("waitForInitWrite did not notice the dead init")
	}
}

func TestWaitForInitWriteReadable(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()

	if _, err := w.Write([]byte("0")); err != nil {
		t.Fatal(err)
	}
	// A readable fd has to win over the liveness check, even for a pid that
	// is long gone.
	if err := waitForInitWrite(int(r.Fd()), 1); err != nil {
		t.Fatalf("waitForInitWrite: %v", err)
	}
}

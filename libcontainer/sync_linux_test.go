package libcontainer

import (
	"os"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestParseSyncDeadChildDoesNotHang reproduces opencontainers/runc#5087:
// parseSync must not block forever if the child dies mid-handshake (e.g.
// killed by seccomp) while its end of the socket is still held open
// elsewhere.
func TestParseSyncDeadChildDoesNotHang(t *testing.T) {
	// Real SOCK_SEQPACKET pair, same as runc's actual sync socket.
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	// Keep a duplicate of the child's end open for the test's lifetime, so
	// the kernel never delivers EOF on fds[0] no matter what happens to
	// the child process below (simulating a leaked reference elsewhere).
	leaked, err := unix.Dup(fds[1])
	if err != nil {
		t.Fatalf("dup: %v", err)
	}
	defer unix.Close(leaked)
	if err := unix.Close(fds[1]); err != nil {
		t.Fatalf("close: %v", err)
	}

	parentFile := os.NewFile(uintptr(fds[0]), "sync-p")
	defer parentFile.Close()
	pipe := newSyncSocket(parentFile)

	// A real child process we can kill, standing in for the container init
	// that gets killed by seccomp mid-handshake.
	cmd := exec.Command("sleep", "100")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	done := make(chan error, 1)
	go func() {
		done <- parseSync(pipe, cmd.Process.Pid, func(sync *syncT) error {
			return nil
		})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected parseSync to return an error when the child dies mid-handshake, got nil")
		}
		t.Logf("parseSync correctly reported the dead child: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("parseSync hung indefinitely after child died with a leaked socket reference (runc#5087)")
	}
}

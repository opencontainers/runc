package integration

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"

	"golang.org/x/sys/unix"
)

func TestPreExecSignalMapping(t *testing.T) {
	if testing.Short() {
		return
	}

	testCases := []struct {
		name string
		sig  unix.Signal
	}{
		{name: "SIGTERM", sig: unix.SIGTERM},
		{name: "SIGINT", sig: unix.SIGINT},
		{name: "SIGHUP", sig: unix.SIGHUP},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			container, process, hookRelease := newPreExecSignalProcess(t)
			defer destroyContainer(container)

			ok(t, container.Signal(tc.sig))
			if got, want := waitForExitCodeWhileHookBlocked(t, process, hookRelease), 128+int(tc.sig); got != want {
				t.Fatalf("expected %s to exit %d, got %d", tc.name, want, got)
			}
		})
	}
}

func newPreExecSignalProcess(t testing.TB) (*libcontainer.Container, *libcontainer.Process, string) {
	t.Helper()

	config := newTemplateConfig(t, nil)
	hookReady := filepath.Join(config.Rootfs, "startContainer-hook-ready")
	hookRelease := filepath.Join(config.Rootfs, "startContainer-hook-release")
	ok(t, unix.Mkfifo(hookRelease, 0o600))
	config.Hooks = configs.Hooks{
		configs.StartContainer: configs.HookList{
			configs.NewCommandHook(&configs.Command{
				Path: "/bin/sh",
				Args: []string{"/bin/sh", "-c", "touch /startContainer-hook-ready && cat /startContainer-hook-release >/dev/null"},
			}),
		},
	}

	container, err := newContainer(t, config)
	ok(t, err)

	process := &libcontainer.Process{
		Cwd:  "/",
		Args: []string{"false"},
		Env:  standardEnvironment,
		Init: true,
	}

	ok(t, container.Run(process))
	waitForFile(t, hookReady)
	return container, process, hookRelease
}

func waitForFile(t testing.TB, path string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat %s: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s", path)
}

func processExitCode(process *libcontainer.Process) (int, error) {
	ps, err := process.Wait()
	if err != nil && ps == nil {
		return 0, err
	}
	if ps == nil {
		return 0, errors.New("wait returned nil process state")
	}

	status, ok := ps.Sys().(syscall.WaitStatus)
	if !ok {
		return 0, errors.New("unexpected wait status type")
	}
	if !status.Exited() {
		return 0, errors.New("process did not exit")
	}
	return status.ExitStatus(), nil
}

func waitForExitCodeWhileHookBlocked(t testing.TB, process *libcontainer.Process, hookRelease string) int {
	t.Helper()

	type waitResult struct {
		code int
		err  error
	}

	results := make(chan waitResult, 1)
	go func() {
		code, err := processExitCode(process)
		results <- waitResult{code: code, err: err}
	}()

	select {
	case result := <-results:
		releaseHook(t, hookRelease)
		if result.err != nil {
			t.Fatalf("wait: %v", result.err)
		}
		return result.code
	case <-time.After(500 * time.Millisecond):
		releaseHook(t, hookRelease)
		t.Fatal("process did not exit while startContainer hook was still blocking")
		return 0
	}
}

func releaseHook(t testing.TB, path string) {
	t.Helper()

	fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if errors.Is(err, unix.ENXIO) || errors.Is(err, os.ErrNotExist) {
		return
	}
	ok(t, err)
	f := os.NewFile(uintptr(fd), path)
	if _, err := f.Write([]byte("ok\n")); err != nil {
		_ = f.Close()
		t.Fatalf("write %s: %v", path, err)
	}
	ok(t, f.Close())

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if errors.Is(err, unix.ENXIO) || errors.Is(err, os.ErrNotExist) {
			return
		}
		if err == nil {
			_ = unix.Close(fd)
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for startContainer hook to exit")
}

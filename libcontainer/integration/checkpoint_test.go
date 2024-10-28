package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"golang.org/x/sys/unix"
)

func criuFeature(feature string) bool {
	return exec.Command("criu", "check", "--feature", feature).Run() == nil
}

func TestUsernsCheckpoint(t *testing.T) {
	testCheckpoint(t, true)
}

func TestCheckpoint(t *testing.T) {
	testCheckpoint(t, false)
}

func testCheckpoint(t *testing.T, userns bool) {
	if testing.Short() {
		return
	}

	if _, err := exec.LookPath("criu"); err != nil {
		t.Skipf("criu binary not found: %v", err)
	}

	// Workaround for https://github.com/opencontainers/runc/issues/3532.
	out, err := exec.Command("rpm", "-q", "criu").CombinedOutput()
	if err == nil && regexp.MustCompile(`^criu-3\.17-[123]\.el9`).Match(out) {
		t.Skip("Test requires criu >= 3.17-4 on CentOS Stream 9.")
	}

	if userns && !criuFeature("userns") {
		t.Skip("Test requires userns")
	}

	config := newTemplateConfig(t, &tParam{userns: userns})
	stateDir := t.TempDir()

	container, err := libcontainer.Create(stateDir, "test", config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	var stdout bytes.Buffer

	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"cat"},
		Env:    standardEnvironment,
		Stdin:  stdinR,
		Stdout: &stdout,
		Init:   true,
	}

	err = container.Run(&pconfig)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	pid, err := pconfig.Pid()
	ok(t, err)

	process, err := os.FindProcess(pid)
	ok(t, err)

	tmp := t.TempDir()
	var parentImage string

	retryCheckpoint := func(opts *libcontainer.CriuOpts) error {
		err := container.Checkpoint(opts)
		// Cgroup v1 freezer is flaky; v2 is fine.
		if err == nil || cgroups.IsCgroup2UnifiedMode() {
			return err
		}

		const retries = 2
		for i := 1; i <= retries; i++ {
			time.Sleep(time.Second << i)
			t.Logf("cgroup v1 checkpointing is flaky, retry %d of %d", i, retries)
			if err = container.Checkpoint(opts); err == nil {
				return nil
			}
		}
		return err
	}

	// Test pre-dump if mem_dirty_track is available.
	if criuFeature("mem_dirty_track") {
		parentImage = "../criu-parent"
		parentDir := filepath.Join(tmp, "criu-parent")
		preDumpOpts := &libcontainer.CriuOpts{
			ImagesDirectory: parentDir,
			WorkDirectory:   parentDir,
			PreDump:         true,
		}

		if err := retryCheckpoint(preDumpOpts); err != nil {
			t.Fatal(err)
		}

		state, err := container.Status()
		ok(t, err)

		if state != libcontainer.Running {
			t.Fatal("Unexpected preDump state: ", state)
		}
	}

	imagesDir := filepath.Join(tmp, "criu")

	checkpointOpts := &libcontainer.CriuOpts{
		ImagesDirectory: imagesDir,
		WorkDirectory:   imagesDir,
		ParentImage:     parentImage,
	}

	if err := retryCheckpoint(checkpointOpts); err != nil {
		t.Fatal(err)
	}

	state, err := container.Status()
	ok(t, err)

	if state != libcontainer.Stopped {
		t.Fatal("Unexpected state checkpoint: ", state)
	}

	_ = stdinW.Close()
	_, err = process.Wait()
	ok(t, err)

	// reload the container
	container, err = libcontainer.Load(stateDir, "test")
	ok(t, err)

	restoreStdinR, restoreStdinW, err := os.Pipe()
	ok(t, err)

	var restoreStdout bytes.Buffer
	restoreProcessConfig := &libcontainer.Process{
		Cwd:    "/",
		Stdin:  restoreStdinR,
		Stdout: &restoreStdout,
		Init:   true,
	}

	err = container.Restore(restoreProcessConfig, checkpointOpts)
	_ = restoreStdinR.Close()
	defer restoreStdinW.Close() //nolint: errcheck
	if err != nil {
		t.Fatal(err)
	}

	state, err = container.Status()
	ok(t, err)
	if state != libcontainer.Running {
		t.Fatal("Unexpected restore state: ", state)
	}

	pid, err = restoreProcessConfig.Pid()
	ok(t, err)

	err = unix.Kill(pid, 0)
	ok(t, err)

	_, err = restoreStdinW.WriteString("Hello!")
	ok(t, err)

	_ = restoreStdinW.Close()
	waitProcess(restoreProcessConfig, t)

	output := restoreStdout.String()
	if !strings.Contains(output, "Hello!") {
		t.Fatal("Did not restore the pipe correctly:", output)
	}
}

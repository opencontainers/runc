package integration

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/containerd/console"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"

	"golang.org/x/sys/unix"
)

func TestExecIn(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	defer func() {
		_ = stdinR.Close()
		_ = stdinW.Close()
	}()
	stdoutR, stdoutW, err := os.Pipe()
	ok(t, err)
	defer func() {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
	}()

	ch := waitStdOut(stdoutR)
	process := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"cat", "/proc/self/cmdline", "-"},
		Env:    standardEnvironment,
		Stdin:  stdinR,
		Stdout: stdoutW,
		Init:   true,
	}
	err = container.Run(process)
	defer func() {
		_, _ = stdinW.Write([]byte("hello"))
		_ = stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	ok(t, err)

	err = <-ch
	ok(t, err)

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"ps"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}

	err = container.Run(ps)
	ok(t, err)
	waitProcess(ps, t)

	out := buffers.Stdout.String()
	if !strings.Contains(out, "cat") || !strings.Contains(out, "ps") {
		t.Fatalf("unexpected running process, output %q", out)
	}
	if strings.Contains(out, "\r") {
		t.Fatalf("unexpected carriage-return in output %q", out)
	}
}

func TestExecInUsernsRlimit(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}

	testExecInRlimit(t, true)
}

func TestExecInRlimit(t *testing.T) {
	testExecInRlimit(t, false)
}

func testExecInRlimit(t *testing.T, userns bool) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, &tParam{userns: userns})
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"/bin/sh", "-c", "ulimit -n"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
		Rlimits: []configs.Rlimit{
			// increase process rlimit higher than container rlimit to test per-process limit
			{Type: unix.RLIMIT_NOFILE, Hard: 1026, Soft: 1026},
		},
	}
	err = container.Run(ps)
	ok(t, err)
	waitProcess(ps, t)

	_ = stdinW.Close()
	waitProcess(process, t)

	out := buffers.Stdout.String()
	if limit := strings.TrimSpace(out); limit != "1026" {
		t.Fatalf("expected rlimit to be 1026, got %s", limit)
	}
}

func TestExecInAdditionalGroups(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:              "/",
		Args:             []string{"sh", "-c", "id", "-Gn"},
		Env:              standardEnvironment,
		Stdin:            nil,
		Stdout:           &stdout,
		AdditionalGroups: []int{4444, 87654},
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	_ = stdinW.Close()
	waitProcess(process, t)

	outputGroups := stdout.String()

	// Check that the groups output has the groups that we specified.
	for _, gid := range pconfig.AdditionalGroups {
		if !strings.Contains(outputGroups, strconv.Itoa(gid)) {
			t.Errorf("Listed groups do not contain gid %d as expected: %v", gid, outputGroups)
		}
	}
}

func TestExecInError(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer func() {
		_ = stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	ok(t, err)

	for i := 0; i < 42; i++ {
		unexistent := &libcontainer.Process{
			Cwd:  "/",
			Args: []string{"unexistent"},
			Env:  standardEnvironment,
		}
		err = container.Run(unexistent)
		if err == nil {
			t.Fatal("Should be an error")
		}
		if !strings.Contains(err.Error(), "executable file not found") {
			t.Fatalf("Should be error about not found executable, got %s", err)
		}
	}
}

func TestExecInTTY(t *testing.T) {
	if testing.Short() {
		return
	}
	t.Skip("racy; see https://github.com/opencontainers/runc/issues/2425")
	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	defer func() {
		_ = stdinR.Close()
		_ = stdinW.Close()
	}()
	stdoutR, stdoutW, err := os.Pipe()
	ok(t, err)
	defer func() {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
	}()

	ch := waitStdOut(stdoutR)
	process := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"cat", "/proc/self/cmdline", "-"},
		Env:    standardEnvironment,
		Stdin:  stdinR,
		Stdout: stdoutW,
		Init:   true,
	}
	err = container.Run(process)
	defer func() {
		_, _ = stdinW.Write([]byte("hello"))
		_ = stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	ok(t, err)

	err = <-ch
	ok(t, err)

	ps := &libcontainer.Process{
		Cwd:  "/",
		Args: []string{"ps"},
		Env:  standardEnvironment,
	}

	// Repeat to increase chances to catch a race; see
	// https://github.com/opencontainers/runc/issues/2425.
	for i := 0; i < 300; i++ {
		var stdout bytes.Buffer

		parent, child, err := utils.NewSockPair("console")
		ok(t, err)
		ps.ConsoleSocket = child

		done := make(chan (error))
		go func() {
			f, err := utils.RecvFile(parent)
			if err != nil {
				done <- fmt.Errorf("RecvFile: %w", err)
				return
			}
			c, err := console.ConsoleFromFile(f)
			if err != nil {
				done <- fmt.Errorf("ConsoleFromFile: %w", err)
				return
			}
			err = console.ClearONLCR(c.Fd())
			if err != nil {
				done <- fmt.Errorf("ClearONLCR: %w", err)
				return
			}
			// An error from io.Copy is expected once the terminal
			// is gone, so we deliberately ignore it.
			_, _ = io.Copy(&stdout, c)
			done <- nil
		}()

		err = container.Run(ps)
		ok(t, err)

		select {
		case <-time.After(5 * time.Second):
			t.Fatal("Waiting for copy timed out")
		case err := <-done:
			ok(t, err)
		}

		waitProcess(ps, t)
		_ = parent.Close()
		_ = child.Close()

		out := stdout.String()
		if !strings.Contains(out, "cat") || !strings.Contains(out, "ps") {
			t.Fatalf("unexpected running process, output %q", out)
		}
		if strings.Contains(out, "\r") {
			t.Fatalf("unexpected carriage-return in output %q", out)
		}
	}
}

func TestExecInEnvironment(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	execEnv := []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		// The below line is added to check the deduplication feature:
		// if a variable with the same name appears more than once,
		// only the last value should be added to the environment.
		"DEBUG=true",
		"DEBUG=false",
		"ENV=test",
	}
	buffers := newStdBuffers()
	process2 := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"/bin/env"},
		Env:    execEnv,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Run(process2)
	ok(t, err)
	waitProcess(process2, t)

	_ = stdinW.Close()
	waitProcess(process, t)

	// Check exec process environment.
	t.Logf("exec output:\n%s", buffers.Stdout.String())
	out := strings.Fields(buffers.Stdout.String())
	// If not present in the Process.Env, runc should add $HOME,
	// which is deduced by parsing container's /etc/passwd.
	// We use a known image in the test, so we know its value.
	for _, e := range append(execEnv, "HOME=/root") {
		if e == "DEBUG=true" { // This one should be dedup'ed out.
			continue
		}
		if !slices.Contains(out, e) {
			t.Errorf("Expected env %s not found in exec output", e)
		}
	}
	// Check that there are no extra variables. We have 1 env ("DEBUG=true")
	// removed and 1 env ("HOME=root") added, resulting in the same number.
	if got, want := len(out), len(execEnv); want != got {
		t.Errorf("Mismatched number of variables: want %d, got %d", want, got)
	}
}

func TestExecinPassExtraFiles(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	var stdout bytes.Buffer
	pipeout1, pipein1, err := os.Pipe()
	ok(t, err)
	pipeout2, pipein2, err := os.Pipe()
	ok(t, err)
	inprocess := &libcontainer.Process{
		Cwd:        "/",
		Args:       []string{"sh", "-c", "cd /proc/$$/fd; echo -n *; echo -n 1 >3; echo -n 2 >4"},
		Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		ExtraFiles: []*os.File{pipein1, pipein2},
		Stdin:      nil,
		Stdout:     &stdout,
	}
	err = container.Run(inprocess)
	ok(t, err)

	waitProcess(inprocess, t)
	_ = stdinW.Close()
	waitProcess(process, t)

	out := stdout.String()
	// fd 5 is the directory handle for /proc/$$/fd
	if out != "0 1 2 3 4 5" {
		t.Fatalf("expected to have the file descriptors '0 1 2 3 4 5' passed to exec, got '%s'", out)
	}
	buf := []byte{0}
	_, err = pipeout1.Read(buf)
	ok(t, err)
	out1 := string(buf)
	if out1 != "1" {
		t.Fatalf("expected first pipe to receive '1', got '%s'", out1)
	}

	_, err = pipeout2.Read(buf)
	ok(t, err)
	out2 := string(buf)
	if out2 != "2" {
		t.Fatalf("expected second pipe to receive '2', got '%s'", out2)
	}
}

func TestExecInOomScoreAdj(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	config.OomScoreAdj = ptrInt(200)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)
	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"/bin/sh", "-c", "cat /proc/self/oom_score_adj"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Run(ps)
	ok(t, err)
	waitProcess(ps, t)

	_ = stdinW.Close()
	waitProcess(process, t)

	out := buffers.Stdout.String()
	if oomScoreAdj := strings.TrimSpace(out); oomScoreAdj != strconv.Itoa(*config.OomScoreAdj) {
		t.Fatalf("expected oomScoreAdj to be %d, got %s", *config.OomScoreAdj, oomScoreAdj)
	}
}

func TestExecInUserns(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, &tParam{userns: true})
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	process := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(process)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	initPID, err := process.Pid()
	ok(t, err)
	initUserns, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/user", initPID))
	ok(t, err)

	buffers := newStdBuffers()
	process2 := &libcontainer.Process{
		Cwd:  "/",
		Args: []string{"readlink", "/proc/self/ns/user"},
		Env: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		},
		Stdout: buffers.Stdout,
		Stderr: os.Stderr,
	}
	err = container.Run(process2)
	ok(t, err)
	waitProcess(process2, t)
	_ = stdinW.Close()
	waitProcess(process, t)

	if out := strings.TrimSpace(buffers.Stdout.String()); out != initUserns {
		t.Errorf("execin userns(%s), wanted %s", out, initUserns)
	}
}

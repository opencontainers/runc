package integration

import (
	"os"
	"strings"
	"testing"

	"github.com/docker/libcontainer"
)

func TestExecIn(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Args:   []string{"ps"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Start(ps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if !strings.Contains(out, "cat") || !strings.Contains(out, "ps") {
		t.Fatalf("unexpected running process, output %q", out)
	}
}

func TestExecInRlimit(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Args:   []string{"/bin/sh", "-c", "ulimit -n"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	err = container.Start(ps)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	stdinW.Close()
	if _, err := process.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if limit := strings.TrimSpace(out); limit != "1024" {
		t.Fatalf("expected rlimit to be 1024, got %s", limit)
	}
}

func TestExecInError(t *testing.T) {
	if testing.Short() {
		return
	}
	rootfs, err := newRootfs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)
	config := newTemplateConfig(rootfs)
	container, err := newContainer(config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &libcontainer.Process{
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
	}
	err = container.Start(process)
	stdinR.Close()
	defer func() {
		stdinW.Close()
		if _, err := process.Wait(); err != nil {
			t.Log(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}

	unexistent := &libcontainer.Process{
		Args: []string{"unexistent"},
		Env:  standardEnvironment,
	}
	err = container.Start(unexistent)
	if err == nil {
		t.Fatal("Should be an error")
	}
	if !strings.Contains(err.Error(), "executable file not found") {
		t.Fatalf("Should be error about not found executable, got %s", err)
	}
}

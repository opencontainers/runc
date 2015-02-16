package integration

import (
	"os"
	"strings"
	"syscall"
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
	buffers := newStdBuffers()
	process := &libcontainer.Process{
		Args:   []string{"sleep", "10"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	pid1, err := container.Start(process)
	if err != nil {
		t.Fatal(err)
	}
	buffers = newStdBuffers()
	psPid, err := container.Start(&libcontainer.Process{
		Args:   []string{"ps"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	ps, err := os.FindProcess(psPid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	p, err := os.FindProcess(pid1)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Signal(syscall.SIGKILL); err != nil {
		t.Log(err)
	}
	if _, err := p.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if !strings.Contains(out, "sleep 10") || !strings.Contains(out, "ps") {
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
	buffers := newStdBuffers()
	process := &libcontainer.Process{
		Args:   []string{"sleep", "10"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	}
	pid1, err := container.Start(process)
	if err != nil {
		t.Fatal(err)
	}
	buffers = newStdBuffers()
	psPid, err := container.Start(&libcontainer.Process{
		Args:   []string{"/bin/sh", "-c", "ulimit -n"},
		Env:    standardEnvironment,
		Stdin:  buffers.Stdin,
		Stdout: buffers.Stdout,
		Stderr: buffers.Stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	ps, err := os.FindProcess(psPid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ps.Wait(); err != nil {
		t.Fatal(err)
	}
	p, err := os.FindProcess(pid1)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Signal(syscall.SIGKILL); err != nil {
		t.Log(err)
	}
	if _, err := p.Wait(); err != nil {
		t.Log(err)
	}
	out := buffers.Stdout.String()
	if limit := strings.TrimSpace(out); limit != "1024" {
		t.Fatalf("expected rlimit to be 1024, got %s", limit)
	}
}

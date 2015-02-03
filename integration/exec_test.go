package integration

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
)

func TestExecPS(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)
	buffers, exitCode, err := runContainer(config, "", "ps")
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 0 {
		t.Fatalf("exit code not 0. code %d stderr %q", exitCode, buffers.Stderr)
	}

	lines := strings.Split(buffers.Stdout.String(), "\n")
	if len(lines) < 2 {
		t.Fatalf("more than one process running for output %q", buffers.Stdout.String())
	}
	expected := `1 root     ps`
	actual := strings.Trim(lines[1], "\n ")
	if actual != expected {
		t.Fatalf("expected output %q but received %q", expected, actual)
	}
}

func TestIPCPrivate(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	l, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	config := newTemplateConfig(rootfs)
	buffers, exitCode, err := runContainer(config, "", "readlink", "/proc/self/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 0 {
		t.Fatalf("exit code not 0. code %d stderr %q", exitCode, buffers.Stderr)
	}

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual == l {
		t.Fatalf("ipc link should be private to the container but equals host %q %q", actual, l)
	}
}

func TestIPCHost(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	l, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	config := newTemplateConfig(rootfs)
	config.Namespaces.Remove(configs.NEWIPC)
	buffers, exitCode, err := runContainer(config, "", "readlink", "/proc/self/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 0 {
		t.Fatalf("exit code not 0. code %d stderr %q", exitCode, buffers.Stderr)
	}

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("ipc link not equal to host link %q %q", actual, l)
	}
}

func TestIPCJoinPath(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	l, err := os.Readlink("/proc/1/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	config := newTemplateConfig(rootfs)
	config.Namespaces.Add(configs.NEWIPC, "/proc/1/ns/ipc")

	buffers, exitCode, err := runContainer(config, "", "readlink", "/proc/self/ns/ipc")
	if err != nil {
		t.Fatal(err)
	}

	if exitCode != 0 {
		t.Fatalf("exit code not 0. code %d stderr %q", exitCode, buffers.Stderr)
	}

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("ipc link not equal to host link %q %q", actual, l)
	}
}

func TestIPCBadPath(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)
	config.Namespaces.Add(configs.NEWIPC, "/proc/1/ns/ipcc")

	_, _, err = runContainer(config, "", "true")
	if err == nil {
		t.Fatal("container succeeded with bad ipc path")
	}
}

func TestRlimit(t *testing.T) {
	if testing.Short() {
		return
	}

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)
	out, _, err := runContainer(config, "", "/bin/sh", "-c", "ulimit -n")
	if err != nil {
		t.Fatal(err)
	}
	if limit := strings.TrimSpace(out.Stdout.String()); limit != "1024" {
		t.Fatalf("expected rlimit to be 1024, got %s", limit)
	}
}

func newTestRoot() (string, error) {
	dir, err := ioutil.TempDir("", "libcontainer")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func waitProcess(pid int, t *testing.T) {
	p, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	status, err := p.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Success() {
		t.Fatal(status)
	}
}

func TestEnter(t *testing.T) {
	if testing.Short() {
		return
	}
	root, err := newTestRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)

	factory, err := libcontainer.New(root, []string{os.Args[0], "init", "--"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := factory.Create("test", config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	// Execute a first process in the container
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stdout2 bytes.Buffer

	pconfig := libcontainer.Process{
		Args:   []string{"sh", "-c", "cat && readlink /proc/self/ns/pid"},
		Stdin:  stdinR,
		Stdout: &stdout,
	}
	pid, err := container.Start(&pconfig)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Execute a first process in the container
	stdinR2, stdinW2, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	pconfig.Args = []string{"sh", "-c", "cat && readlink /proc/self/ns/pid"}
	pconfig.Stdin = stdinR2
	pconfig.Stdout = &stdout2

	pid2, err := container.Start(&pconfig)
	stdinR2.Close()
	defer stdinW2.Close()
	if err != nil {
		t.Fatal(err)
	}

	processes, err := container.Processes()
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	for i := range processes {
		if processes[i] == pid || processes[i] == pid2 {
			n++
		}
	}
	if n != 2 {
		t.Fatal("unexpected number of processes", processes, pid, pid2)
	}

	// Wait processes
	stdinW2.Close()
	waitProcess(pid2, t)

	stdinW.Close()
	waitProcess(pid, t)

	// Check that both processes live in the same pidns
	pidns := string(stdout.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	pidns2 := string(stdout2.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if pidns != pidns2 {
		t.Fatal("The second process isn't in the required pid namespace", pidns, pidns2)
	}
}

func TestFreeze(t *testing.T) {
	if testing.Short() {
		return
	}
	root, err := newTestRoot()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	rootfs, err := newRootFs()
	if err != nil {
		t.Fatal(err)
	}
	defer remove(rootfs)

	config := newTemplateConfig(rootfs)

	factory, err := libcontainer.New(root, []string{os.Args[0], "init", "--"})
	if err != nil {
		t.Fatal(err)
	}

	container, err := factory.Create("test", config)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Destroy()

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	pconfig := libcontainer.Process{
		Args:  []string{"cat"},
		Stdin: stdinR,
	}
	pid, err := container.Start(&pconfig)
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}

	if err := container.Pause(); err != nil {
		t.Fatal(err)
	}
	state, err := container.Status()
	if err != nil {
		t.Fatal(err)
	}
	if err := container.Resume(); err != nil {
		t.Fatal(err)
	}
	if state != configs.Paused {
		t.Fatal("Unexpected state: ", state)
	}

	stdinW.Close()
	s, err := process.Wait()
	if err != nil {
		t.Fatal(err)
	}
	if !s.Success() {
		t.Fatal(s.String())
	}
}

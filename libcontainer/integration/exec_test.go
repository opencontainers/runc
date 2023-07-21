package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"

	"golang.org/x/sys/unix"
)

func TestExecPS(t *testing.T) {
	testExecPS(t, false)
}

func TestUsernsExecPS(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}
	testExecPS(t, true)
}

func testExecPS(t *testing.T, userns bool) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, &tParam{userns: userns})

	buffers := runContainerOk(t, config, "ps", "-o", "pid,user,comm")
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

	l, err := os.Readlink("/proc/1/ns/ipc")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/ipc")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual == l {
		t.Fatalf("ipc link should be private to the container but equals host %q %q", actual, l)
	}
}

func TestIPCHost(t *testing.T) {
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/ipc")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	config.Namespaces.Remove(configs.NEWIPC)
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/ipc")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("ipc link not equal to host link %q %q", actual, l)
	}
}

func TestIPCJoinPath(t *testing.T) {
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/ipc")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	config.Namespaces.Add(configs.NEWIPC, "/proc/1/ns/ipc")
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/ipc")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("ipc link not equal to host link %q %q", actual, l)
	}
}

func TestIPCBadPath(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	config.Namespaces.Add(configs.NEWIPC, "/proc/1/ns/ipcc")

	if _, _, err := runContainer(t, config, "true"); err == nil {
		t.Fatal("container succeeded with bad ipc path")
	}
}

func TestRlimit(t *testing.T) {
	testRlimit(t, false)
}

func TestUsernsRlimit(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}

	testRlimit(t, true)
}

func testRlimit(t *testing.T, userns bool) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, &tParam{userns: userns})

	// ensure limit is lower than what the config requests to test that in a user namespace
	// the Setrlimit call happens early enough that we still have permissions to raise the limit.
	ok(t, unix.Setrlimit(unix.RLIMIT_NOFILE, &unix.Rlimit{
		Max: 1024,
		Cur: 1024,
	}))

	out := runContainerOk(t, config, "/bin/sh", "-c", "ulimit -n")
	if limit := strings.TrimSpace(out.Stdout.String()); limit != "1025" {
		t.Fatalf("expected rlimit to be 1025, got %s", limit)
	}
}

func TestEnter(t *testing.T) {
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

	var stdout, stdout2 bytes.Buffer

	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"sh", "-c", "cat && readlink /proc/self/ns/pid"},
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

	// Execute another process in the container
	stdinR2, stdinW2, err := os.Pipe()
	ok(t, err)
	pconfig2 := libcontainer.Process{
		Cwd: "/",
		Env: standardEnvironment,
	}
	pconfig2.Args = []string{"sh", "-c", "cat && readlink /proc/self/ns/pid"}
	pconfig2.Stdin = stdinR2
	pconfig2.Stdout = &stdout2

	err = container.Run(&pconfig2)
	_ = stdinR2.Close()
	defer stdinW2.Close() //nolint: errcheck
	ok(t, err)

	pid2, err := pconfig2.Pid()
	ok(t, err)

	processes, err := container.Processes()
	ok(t, err)

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
	_ = stdinW2.Close()
	waitProcess(&pconfig2, t)

	_ = stdinW.Close()
	waitProcess(&pconfig, t)

	// Check that both processes live in the same pidns
	pidns := stdout.String()
	ok(t, err)

	pidns2 := stdout2.String()
	ok(t, err)

	if pidns != pidns2 {
		t.Fatal("The second process isn't in the required pid namespace", pidns, pidns2)
	}
}

func TestProcessEnv(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:  "/",
		Args: []string{"sh", "-c", "env"},
		Env: []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"HOSTNAME=integration",
			"TERM=xterm",
			"FOO=BAR",
		},
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	outputEnv := stdout.String()

	// Check that the environment has the key/value pair we added
	if !strings.Contains(outputEnv, "FOO=BAR") {
		t.Fatal("Environment doesn't have the expected FOO=BAR key/value pair: ", outputEnv)
	}

	// Make sure that HOME is set
	if !strings.Contains(outputEnv, "HOME=/root") {
		t.Fatal("Environment doesn't have HOME set: ", outputEnv)
	}
}

func TestProcessEmptyCaps(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	config.Capabilities = nil

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"sh", "-c", "cat /proc/self/status"},
		Env:    standardEnvironment,
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	outputStatus := stdout.String()

	lines := strings.Split(outputStatus, "\n")

	effectiveCapsLine := ""
	for _, l := range lines {
		line := strings.TrimSpace(l)
		if strings.Contains(line, "CapEff:") {
			effectiveCapsLine = line
			break
		}
	}

	if effectiveCapsLine == "" {
		t.Fatal("Couldn't find effective caps: ", outputStatus)
	}
}

func TestProcessCaps(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:          "/",
		Args:         []string{"sh", "-c", "cat /proc/self/status"},
		Env:          standardEnvironment,
		Stdin:        nil,
		Stdout:       &stdout,
		Capabilities: &configs.Capabilities{},
		Init:         true,
	}
	pconfig.Capabilities.Bounding = append(config.Capabilities.Bounding, "CAP_NET_ADMIN")
	pconfig.Capabilities.Permitted = append(config.Capabilities.Permitted, "CAP_NET_ADMIN")
	pconfig.Capabilities.Effective = append(config.Capabilities.Effective, "CAP_NET_ADMIN")
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	outputStatus := stdout.String()

	lines := strings.Split(outputStatus, "\n")

	effectiveCapsLine := ""
	for _, l := range lines {
		line := strings.TrimSpace(l)
		if strings.Contains(line, "CapEff:") {
			effectiveCapsLine = line
			break
		}
	}

	if effectiveCapsLine == "" {
		t.Fatal("Couldn't find effective caps: ", outputStatus)
	}

	parts := strings.Split(effectiveCapsLine, ":")
	effectiveCapsStr := strings.TrimSpace(parts[1])

	effectiveCaps, err := strconv.ParseUint(effectiveCapsStr, 16, 64)
	if err != nil {
		t.Fatal("Could not parse effective caps", err)
	}

	const netAdminMask = 1 << unix.CAP_NET_ADMIN
	if effectiveCaps&netAdminMask != netAdminMask {
		t.Fatal("CAP_NET_ADMIN is not set as expected")
	}
}

func TestAdditionalGroups(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:              "/",
		Args:             []string{"sh", "-c", "id", "-Gn"},
		Env:              standardEnvironment,
		Stdin:            nil,
		Stdout:           &stdout,
		AdditionalGroups: []string{"plugdev", "audio"},
		Init:             true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	outputGroups := stdout.String()

	// Check that the groups output has the groups that we specified
	if !strings.Contains(outputGroups, "audio") {
		t.Fatalf("Listed groups do not contain the audio group as expected: %v", outputGroups)
	}

	if !strings.Contains(outputGroups, "plugdev") {
		t.Fatalf("Listed groups do not contain the plugdev group as expected: %v", outputGroups)
	}
}

func TestFreeze(t *testing.T) {
	for _, systemd := range []bool{true, false} {
		for _, set := range []bool{true, false} {
			name := ""
			if systemd {
				name += "Systemd"
			} else {
				name += "FS"
			}
			if set {
				name += "ViaSet"
			} else {
				name += "ViaPauseResume"
			}
			t.Run(name, func(t *testing.T) {
				testFreeze(t, systemd, set)
			})
		}
	}
}

func testFreeze(t *testing.T, withSystemd bool, useSet bool) {
	if testing.Short() {
		return
	}
	if withSystemd && !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}

	config := newTemplateConfig(t, &tParam{systemd: withSystemd})
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	pconfig := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(pconfig)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	if !useSet {
		err = container.Pause()
	} else {
		config.Cgroups.Resources.Freezer = configs.Frozen
		err = container.Set(*config)
	}
	ok(t, err)

	state, err := container.Status()
	ok(t, err)
	if state != libcontainer.Paused {
		t.Fatal("Unexpected state: ", state)
	}

	if !useSet {
		err = container.Resume()
	} else {
		config.Cgroups.Resources.Freezer = configs.Thawed
		err = container.Set(*config)
	}
	ok(t, err)

	_ = stdinW.Close()
	waitProcess(pconfig, t)
}

func TestCpuShares(t *testing.T) {
	testCpuShares(t, false)
}

func TestCpuSharesSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testCpuShares(t, true)
}

func testCpuShares(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("cgroup v2 does not support CpuShares")
	}

	config := newTemplateConfig(t, &tParam{systemd: systemd})
	config.Cgroups.Resources.CpuShares = 1

	if _, _, err := runContainer(t, config, "ps"); err == nil {
		t.Fatal("runContainer should fail with invalid CpuShares")
	}
}

func TestPids(t *testing.T) {
	testPids(t, false)
}

func TestPidsSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testPids(t, true)
}

func testPids(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, &tParam{systemd: systemd})
	config.Cgroups.Resources.PidsLimit = -1

	// Running multiple processes, expecting it to succeed with no pids limit.
	_ = runContainerOk(t, config, "/bin/sh", "-c", "/bin/true | /bin/true | /bin/true | /bin/true")

	// Enforce a permissive limit. This needs to be fairly hand-wavey due to the
	// issues with running Go binaries with pids restrictions (see below).
	config.Cgroups.Resources.PidsLimit = 64
	_ = runContainerOk(t, config, "/bin/sh", "-c", `
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true`)

	// Enforce a restrictive limit. 64 * /bin/true + 1 * shell should cause
	// this to fail reliably.
	config.Cgroups.Resources.PidsLimit = 64
	out, _, err := runContainer(t, config, "/bin/sh", "-c", `
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true |
	/bin/true | /bin/true | /bin/true | /bin/true | /bin/true | /bin/true | bin/true | /bin/true`)
	if err != nil && !strings.Contains(out.String(), "sh: can't fork") {
		t.Fatal(err)
	}

	if err == nil {
		t.Fatal("expected fork() to fail with restrictive pids limit")
	}

	// Minimal restrictions are not really supported, due to quirks in using Go
	// due to the fact that it spawns random processes. While we do our best with
	// late setting cgroup values, it's just too unreliable with very small pids.max.
	// As such, we don't test that case. YMMV.
}

func TestCgroupResourcesUnifiedErrorOnV1(t *testing.T) {
	testCgroupResourcesUnifiedErrorOnV1(t, false)
}

func TestCgroupResourcesUnifiedErrorOnV1Systemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testCgroupResourcesUnifiedErrorOnV1(t, true)
}

func testCgroupResourcesUnifiedErrorOnV1(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("requires cgroup v1")
	}

	config := newTemplateConfig(t, &tParam{systemd: systemd})
	config.Cgroups.Resources.Unified = map[string]string{
		"memory.min": "10240",
	}
	_, _, err := runContainer(t, config, "true")
	if !strings.Contains(err.Error(), cgroups.ErrV1NoUnified.Error()) {
		t.Fatalf("expected error to contain %v, got %v", cgroups.ErrV1NoUnified, err)
	}
}

func TestCgroupResourcesUnified(t *testing.T) {
	testCgroupResourcesUnified(t, false)
}

func TestCgroupResourcesUnifiedSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testCgroupResourcesUnified(t, true)
}

func testCgroupResourcesUnified(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}
	if !cgroups.IsCgroup2UnifiedMode() {
		t.Skip("requires cgroup v2")
	}

	config := newTemplateConfig(t, &tParam{systemd: systemd})
	config.Cgroups.Resources.Memory = 536870912     // 512M
	config.Cgroups.Resources.MemorySwap = 536870912 // 512M, i.e. no swap
	config.Namespaces.Add(configs.NEWCGROUP, "")

	testCases := []struct {
		name     string
		cfg      map[string]string
		expError string
		cmd      []string
		exp      string
	}{
		{
			name: "dummy",
			cmd:  []string{"true"},
			exp:  "",
		},
		{
			name: "set memory.min",
			cfg:  map[string]string{"memory.min": "131072"},
			cmd:  []string{"cat", "/sys/fs/cgroup/memory.min"},
			exp:  "131072\n",
		},
		{
			name: "check memory.max",
			cmd:  []string{"cat", "/sys/fs/cgroup/memory.max"},
			exp:  strconv.Itoa(int(config.Cgroups.Resources.Memory)) + "\n",
		},

		{
			name: "overwrite memory.max",
			cfg:  map[string]string{"memory.max": "268435456"},
			cmd:  []string{"cat", "/sys/fs/cgroup/memory.max"},
			exp:  "268435456\n",
		},
		{
			name:     "no such controller error",
			cfg:      map[string]string{"privet.vsem": "vam"},
			expError: "controller \"privet\" not available",
		},
		{
			name:     "slash in key error",
			cfg:      map[string]string{"bad/key": "val"},
			expError: "must be a file name (no slashes)",
		},
		{
			name:     "no dot in key error",
			cfg:      map[string]string{"badkey": "val"},
			expError: "must be in the form CONTROLLER.PARAMETER",
		},
		{
			name:     "read-only parameter",
			cfg:      map[string]string{"pids.current": "42"},
			expError: "failed to write",
		},
	}

	for _, tc := range testCases {
		config.Cgroups.Resources.Unified = tc.cfg
		buffers, ret, err := runContainer(t, config, tc.cmd...)
		if tc.expError != "" {
			if err == nil {
				t.Errorf("case %q failed: expected error, got nil", tc.name)
				continue
			}
			if !strings.Contains(err.Error(), tc.expError) {
				t.Errorf("case %q failed: expected error to contain %q, got %q", tc.name, tc.expError, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("case %q failed: expected no error, got %v (command: %v, status: %d, stderr: %q)",
				tc.name, err, tc.cmd, ret, buffers.Stderr.String())
			continue
		}
		if tc.exp != "" {
			out := buffers.Stdout.String()
			if out != tc.exp {
				t.Errorf("expected %q, got %q", tc.exp, out)
			}
		}
	}
}

func TestContainerState(t *testing.T) {
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/ipc")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	config.Namespaces = configs.Namespaces([]configs.Namespace{
		{Type: configs.NEWNS},
		{Type: configs.NEWUTS},
		// host for IPC
		//{Type: configs.NEWIPC},
		{Type: configs.NEWPID},
		{Type: configs.NEWNET},
	})

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	p := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}
	err = container.Run(p)
	ok(t, err)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck

	st, err := container.State()
	ok(t, err)

	l1, err := os.Readlink(st.NamespacePaths[configs.NEWIPC])
	ok(t, err)
	if l1 != l {
		t.Fatal("Container using non-host ipc namespace")
	}
	_ = stdinW.Close()
	waitProcess(p, t)
}

func TestPassExtraFiles(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pipeout1, pipein1, err := os.Pipe()
	ok(t, err)
	pipeout2, pipein2, err := os.Pipe()
	ok(t, err)
	process := libcontainer.Process{
		Cwd:        "/",
		Args:       []string{"sh", "-c", "cd /proc/$$/fd; echo -n *; echo -n 1 >3; echo -n 2 >4"},
		Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
		ExtraFiles: []*os.File{pipein1, pipein2},
		Stdin:      nil,
		Stdout:     &stdout,
		Init:       true,
	}
	err = container.Run(&process)
	ok(t, err)

	waitProcess(&process, t)

	out := stdout.String()
	// fd 5 is the directory handle for /proc/$$/fd
	if out != "0 1 2 3 4 5" {
		t.Fatalf("expected to have the file descriptors '0 1 2 3 4 5' passed to init, got '%s'", out)
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

func TestSysctl(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	config.Sysctl = map[string]string{
		"kernel.shmmni": "8192",
		"kernel/shmmax": "4194304",
	}
	const (
		cmd = "cat shmmni shmmax"
		exp = "8192\n4194304\n"
	)

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:    "/proc/sys/kernel",
		Args:   []string{"sh", "-c", cmd},
		Env:    standardEnvironment,
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	out := stdout.String()
	if out != exp {
		t.Fatalf("expected %s, got %s", exp, out)
	}
}

func TestMountCgroupRO(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	buffers := runContainerOk(t, config, "mount")

	mountInfo := buffers.Stdout.String()
	lines := strings.Split(mountInfo, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "tmpfs on /sys/fs/cgroup") {
			if !strings.Contains(l, "ro") ||
				!strings.Contains(l, "nosuid") ||
				!strings.Contains(l, "nodev") ||
				!strings.Contains(l, "noexec") {
				t.Fatalf("Mode expected to contain 'ro,nosuid,nodev,noexec': %s", l)
			}
			if !strings.Contains(l, "mode=755") {
				t.Fatalf("Mode expected to contain 'mode=755': %s", l)
			}
			continue
		}
		if !strings.HasPrefix(l, "cgroup") {
			continue
		}
		if !strings.Contains(l, "ro") ||
			!strings.Contains(l, "nosuid") ||
			!strings.Contains(l, "nodev") ||
			!strings.Contains(l, "noexec") {
			t.Fatalf("Mode expected to contain 'ro,nosuid,nodev,noexec': %s", l)
		}
	}
}

func TestMountCgroupRW(t *testing.T) {
	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	// clear the RO flag from cgroup mount
	for _, m := range config.Mounts {
		if m.Device == "cgroup" {
			m.Flags = defaultMountFlags
			break
		}
	}

	buffers := runContainerOk(t, config, "mount")

	mountInfo := buffers.Stdout.String()
	lines := strings.Split(mountInfo, "\n")
	for _, l := range lines {
		if strings.HasPrefix(l, "tmpfs on /sys/fs/cgroup") {
			if !strings.Contains(l, "rw") ||
				!strings.Contains(l, "nosuid") ||
				!strings.Contains(l, "nodev") ||
				!strings.Contains(l, "noexec") {
				t.Fatalf("Mode expected to contain 'rw,nosuid,nodev,noexec': %s", l)
			}
			if !strings.Contains(l, "mode=755") {
				t.Fatalf("Mode expected to contain 'mode=755': %s", l)
			}
			continue
		}
		if !strings.HasPrefix(l, "cgroup") {
			continue
		}
		if !strings.Contains(l, "rw") ||
			!strings.Contains(l, "nosuid") ||
			!strings.Contains(l, "nodev") ||
			!strings.Contains(l, "noexec") {
			t.Fatalf("Mode expected to contain 'rw,nosuid,nodev,noexec': %s", l)
		}
	}
}

func TestOomScoreAdj(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	config.OomScoreAdj = ptrInt(200)

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"sh", "-c", "cat /proc/self/oom_score_adj"},
		Env:    standardEnvironment,
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)
	outputOomScoreAdj := strings.TrimSpace(stdout.String())

	// Check that the oom_score_adj matches the value that was set as part of config.
	if outputOomScoreAdj != strconv.Itoa(*config.OomScoreAdj) {
		t.Fatalf("Expected oom_score_adj %d; got %q", *config.OomScoreAdj, outputOomScoreAdj)
	}
}

func TestHook(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	expectedBundle := t.TempDir()
	config.Labels = append(config.Labels, "bundle="+expectedBundle)

	getRootfsFromBundle := func(bundle string) (string, error) {
		f, err := os.Open(filepath.Join(bundle, "config.json"))
		if err != nil {
			return "", err
		}

		var config configs.Config
		if err = json.NewDecoder(f).Decode(&config); err != nil {
			return "", err
		}
		return config.Rootfs, nil
	}
	createFileFromBundle := func(filename, bundle string) error {
		root, err := getRootfsFromBundle(bundle)
		if err != nil {
			return err
		}

		f, err := os.Create(filepath.Join(root, filename))
		if err != nil {
			return err
		}
		return f.Close()
	}

	// Note FunctionHooks can't be serialized to json this means they won't be passed down to the container
	// For CreateContainer and StartContainer which run in the container namespace, this means we need to pass Command Hooks.
	hookFiles := map[configs.HookName]string{
		configs.Prestart:        "prestart",
		configs.CreateRuntime:   "createRuntime",
		configs.CreateContainer: "createContainer",
		configs.StartContainer:  "startContainer",
		configs.Poststart:       "poststart",
	}

	config.Hooks = configs.Hooks{
		configs.Prestart: configs.HookList{
			configs.NewFunctionHook(func(s *specs.State) error {
				if s.Bundle != expectedBundle {
					t.Fatalf("Expected prestart hook bundlePath '%s'; got '%s'", expectedBundle, s.Bundle)
				}
				return createFileFromBundle(hookFiles[configs.Prestart], s.Bundle)
			}),
		},
		configs.CreateRuntime: configs.HookList{
			configs.NewFunctionHook(func(s *specs.State) error {
				if s.Bundle != expectedBundle {
					t.Fatalf("Expected createRuntime hook bundlePath '%s'; got '%s'", expectedBundle, s.Bundle)
				}
				return createFileFromBundle(hookFiles[configs.CreateRuntime], s.Bundle)
			}),
		},
		configs.CreateContainer: configs.HookList{
			configs.NewCommandHook(configs.Command{
				Path: "/bin/bash",
				Args: []string{"/bin/bash", "-c", fmt.Sprintf("touch ./%s", hookFiles[configs.CreateContainer])},
			}),
		},
		configs.StartContainer: configs.HookList{
			configs.NewCommandHook(configs.Command{
				Path: "/bin/sh",
				Args: []string{"/bin/sh", "-c", fmt.Sprintf("touch /%s", hookFiles[configs.StartContainer])},
			}),
		},
		configs.Poststart: configs.HookList{
			configs.NewFunctionHook(func(s *specs.State) error {
				if s.Bundle != expectedBundle {
					t.Fatalf("Expected poststart hook bundlePath '%s'; got '%s'", expectedBundle, s.Bundle)
				}
				return createFileFromBundle(hookFiles[configs.Poststart], s.Bundle)
			}),
		},
		configs.Poststop: configs.HookList{
			configs.NewFunctionHook(func(s *specs.State) error {
				if s.Bundle != expectedBundle {
					t.Fatalf("Expected poststop hook bundlePath '%s'; got '%s'", expectedBundle, s.Bundle)
				}

				root, err := getRootfsFromBundle(s.Bundle)
				if err != nil {
					return err
				}

				for _, hook := range hookFiles {
					if err = os.RemoveAll(filepath.Join(root, hook)); err != nil {
						return err
					}
				}
				return nil
			}),
		},
	}

	// write config of json format into config.json under bundle
	f, err := os.OpenFile(filepath.Join(expectedBundle, "config.json"), os.O_CREATE|os.O_RDWR, 0o644)
	ok(t, err)
	ok(t, json.NewEncoder(f).Encode(config))

	container, err := newContainer(t, config)
	ok(t, err)

	// e.g: 'ls /prestart ...'
	cmd := "ls "
	for _, hook := range hookFiles {
		cmd += "/" + hook + " "
	}

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"sh", "-c", cmd},
		Env:    standardEnvironment,
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	if err := container.Destroy(); err != nil {
		t.Fatalf("container destroy %s", err)
	}

	for _, hook := range []string{"prestart", "createRuntime", "poststart"} {
		fi, err := os.Stat(filepath.Join(config.Rootfs, hook))
		if err == nil || !os.IsNotExist(err) {
			t.Fatalf("expected file '%s to not exists, but it does", fi.Name())
		}
	}
}

func TestSTDIOPermissions(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	buffers := runContainerOk(t, config, "sh", "-c", "echo hi > /dev/stderr")

	if actual := strings.Trim(buffers.Stderr.String(), "\n"); actual != "hi" {
		t.Fatalf("stderr should equal be equal %q %q", actual, "hi")
	}
}

func unmountOp(path string) {
	_ = unix.Unmount(path, unix.MNT_DETACH)
}

// Launch container with rootfsPropagation in rslave mode. Also
// bind mount a volume /mnt1host at /mnt1cont at the time of launch. Now do
// another mount on host (/mnt1host/mnt2host) and this new mount should
// propagate to container (/mnt1cont/mnt2host)
func TestRootfsPropagationSlaveMount(t *testing.T) {
	var mountPropagated bool
	var dir1cont string
	var dir2cont string

	dir1cont = "/root/mnt1cont"

	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	config.RootPropagation = unix.MS_SLAVE | unix.MS_REC

	// Bind mount a volume.
	dir1host := t.TempDir()

	// Make this dir a "shared" mount point. This will make sure a
	// slave relationship can be established in container.
	err := unix.Mount(dir1host, dir1host, "bind", unix.MS_BIND|unix.MS_REC, "")
	ok(t, err)
	err = unix.Mount("", dir1host, "", unix.MS_SHARED|unix.MS_REC, "")
	ok(t, err)
	defer unmountOp(dir1host)

	config.Mounts = append(config.Mounts, &configs.Mount{
		Source:      dir1host,
		Destination: dir1cont,
		Device:      "bind",
		Flags:       unix.MS_BIND | unix.MS_REC,
	})

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	pconfig := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}

	err = container.Run(pconfig)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	// Create mnt2host under dir1host and bind mount itself on top of it.
	// This should be visible in container.
	dir2host := filepath.Join(dir1host, "mnt2host")
	err = os.Mkdir(dir2host, 0o700)
	ok(t, err)
	defer remove(dir2host)

	err = unix.Mount(dir2host, dir2host, "bind", unix.MS_BIND, "")
	defer unmountOp(dir2host)
	ok(t, err)

	// Run "cat /proc/self/mountinfo" in container and look at mount points.
	var stdout2 bytes.Buffer

	stdinR2, stdinW2, err := os.Pipe()
	ok(t, err)

	pconfig2 := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"cat", "/proc/self/mountinfo"},
		Env:    standardEnvironment,
		Stdin:  stdinR2,
		Stdout: &stdout2,
	}

	err = container.Run(pconfig2)
	_ = stdinR2.Close()
	defer stdinW2.Close() //nolint: errcheck
	ok(t, err)

	_ = stdinW2.Close()
	waitProcess(pconfig2, t)
	_ = stdinW.Close()
	waitProcess(pconfig, t)

	mountPropagated = false
	dir2cont = filepath.Join(dir1cont, filepath.Base(dir2host))

	propagationInfo := stdout2.String()
	lines := strings.Split(propagationInfo, "\n")
	for _, l := range lines {
		linefields := strings.Split(l, " ")
		if len(linefields) < 5 {
			continue
		}

		if linefields[4] == dir2cont {
			mountPropagated = true
			break
		}
	}

	if mountPropagated != true {
		t.Fatalf("Mount on host %s did not propagate in container at %s\n", dir2host, dir2cont)
	}
}

// Launch container with rootfsPropagation 0 so no propagation flags are
// applied. Also bind mount a volume /mnt1host at /mnt1cont at the time of
// launch. Now do a mount in container (/mnt1cont/mnt2cont) and this new
// mount should propagate to host (/mnt1host/mnt2cont)

func TestRootfsPropagationSharedMount(t *testing.T) {
	var dir1cont string
	var dir2cont string

	dir1cont = "/root/mnt1cont"

	if testing.Short() {
		return
	}
	config := newTemplateConfig(t, nil)
	config.RootPropagation = unix.MS_PRIVATE

	// Bind mount a volume.
	dir1host := t.TempDir()

	// Make this dir a "shared" mount point. This will make sure a
	// shared relationship can be established in container.
	err := unix.Mount(dir1host, dir1host, "bind", unix.MS_BIND|unix.MS_REC, "")
	ok(t, err)
	err = unix.Mount("", dir1host, "", unix.MS_SHARED|unix.MS_REC, "")
	ok(t, err)
	defer unmountOp(dir1host)

	config.Mounts = append(config.Mounts, &configs.Mount{
		Source:      dir1host,
		Destination: dir1cont,
		Device:      "bind",
		Flags:       unix.MS_BIND | unix.MS_REC,
	})

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	stdinR, stdinW, err := os.Pipe()
	ok(t, err)

	pconfig := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR,
		Init:  true,
	}

	err = container.Run(pconfig)
	_ = stdinR.Close()
	defer stdinW.Close() //nolint: errcheck
	ok(t, err)

	// Create mnt2cont under dir1host. This will become visible inside container
	// at mnt1cont/mnt2cont. Bind mount itself on top of it. This
	// should be visible on host now.
	dir2host := filepath.Join(dir1host, "mnt2cont")
	err = os.Mkdir(dir2host, 0o700)
	ok(t, err)
	defer remove(dir2host)

	dir2cont = filepath.Join(dir1cont, filepath.Base(dir2host))

	// Mount something in container and see if it is visible on host.
	var stdout2 bytes.Buffer

	stdinR2, stdinW2, err := os.Pipe()
	ok(t, err)

	pconfig2 := &libcontainer.Process{
		Cwd:          "/",
		Args:         []string{"mount", "--bind", dir2cont, dir2cont},
		Env:          standardEnvironment,
		Stdin:        stdinR2,
		Stdout:       &stdout2,
		Capabilities: &configs.Capabilities{},
	}

	// Provide CAP_SYS_ADMIN
	pconfig2.Capabilities.Bounding = append(config.Capabilities.Bounding, "CAP_SYS_ADMIN")
	pconfig2.Capabilities.Permitted = append(config.Capabilities.Permitted, "CAP_SYS_ADMIN")
	pconfig2.Capabilities.Effective = append(config.Capabilities.Effective, "CAP_SYS_ADMIN")

	err = container.Run(pconfig2)
	_ = stdinR2.Close()
	defer stdinW2.Close() //nolint: errcheck
	ok(t, err)

	// Wait for process
	_ = stdinW2.Close()
	waitProcess(pconfig2, t)
	_ = stdinW.Close()
	waitProcess(pconfig, t)

	defer unmountOp(dir2host)

	// Check if mount is visible on host or not.
	out, err := exec.Command("findmnt", "-n", "-f", "-oTARGET", dir2host).CombinedOutput()
	outtrim := string(bytes.TrimSpace(out))
	if err != nil {
		t.Logf("findmnt error %q: %q", err, outtrim)
	}

	if outtrim != dir2host {
		t.Fatalf("Mount in container on %s did not propagate to host on %s. finmnt output=%s", dir2cont, dir2host, outtrim)
	}
}

func TestPIDHost(t *testing.T) {
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/pid")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	config.Namespaces.Remove(configs.NEWPID)
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/pid")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("ipc link not equal to host link %q %q", actual, l)
	}
}

func TestHostPidnsInitKill(t *testing.T) {
	config := newTemplateConfig(t, nil)
	// Implicitly use host pid ns.
	config.Namespaces.Remove(configs.NEWPID)
	testPidnsInitKill(t, config)
}

func TestSharedPidnsInitKill(t *testing.T) {
	config := newTemplateConfig(t, nil)
	// Explicitly use host pid ns.
	config.Namespaces.Add(configs.NEWPID, "/proc/1/ns/pid")
	testPidnsInitKill(t, config)
}

func testPidnsInitKill(t *testing.T, config *configs.Config) {
	if testing.Short() {
		return
	}

	// Run a container with two long-running processes.
	container, err := newContainer(t, config)
	ok(t, err)
	defer func() {
		_ = container.Destroy()
	}()

	process1 := &libcontainer.Process{
		Cwd:  "/",
		Args: []string{"sleep", "1h"},
		Env:  standardEnvironment,
		Init: true,
	}
	err = container.Run(process1)
	ok(t, err)

	process2 := &libcontainer.Process{
		Cwd:  "/",
		Args: []string{"sleep", "1h"},
		Env:  standardEnvironment,
		Init: false,
	}
	err = container.Run(process2)
	ok(t, err)

	// Kill the container.
	err = container.Signal(syscall.SIGKILL)
	ok(t, err)
	_, err = process1.Wait()
	if err == nil {
		t.Fatal("expected Wait to indicate failure")
	}

	// The non-init process must've also been killed. If not,
	// the test will time out.
	_, err = process2.Wait()
	if err == nil {
		t.Fatal("expected Wait to indicate failure")
	}
}

func TestInitJoinPID(t *testing.T) {
	if testing.Short() {
		return
	}
	// Execute a long-running container
	config1 := newTemplateConfig(t, nil)
	container1, err := newContainer(t, config1)
	ok(t, err)
	defer destroyContainer(container1)

	stdinR1, stdinW1, err := os.Pipe()
	ok(t, err)
	init1 := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR1,
		Init:  true,
	}
	err = container1.Run(init1)
	_ = stdinR1.Close()
	defer stdinW1.Close() //nolint: errcheck
	ok(t, err)

	// get the state of the first container
	state1, err := container1.State()
	ok(t, err)
	pidns1 := state1.NamespacePaths[configs.NEWPID]

	// Run a container inside the existing pidns but with different cgroups
	config2 := newTemplateConfig(t, nil)
	config2.Namespaces.Add(configs.NEWPID, pidns1)
	config2.Cgroups.Path = "integration/test2"
	container2, err := newContainer(t, config2)
	ok(t, err)
	defer destroyContainer(container2)

	stdinR2, stdinW2, err := os.Pipe()
	ok(t, err)
	init2 := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR2,
		Init:  true,
	}
	err = container2.Run(init2)
	_ = stdinR2.Close()
	defer stdinW2.Close() //nolint: errcheck
	ok(t, err)
	// get the state of the second container
	state2, err := container2.State()
	ok(t, err)

	ns1, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", state1.InitProcessPid))
	ok(t, err)
	ns2, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/pid", state2.InitProcessPid))
	ok(t, err)
	if ns1 != ns2 {
		t.Errorf("pidns(%s), wanted %s", ns2, ns1)
	}

	// check that namespaces are not the same
	if reflect.DeepEqual(state2.NamespacePaths, state1.NamespacePaths) {
		t.Errorf("Namespaces(%v), original %v", state2.NamespacePaths,
			state1.NamespacePaths)
	}
	// check that pidns is joined correctly. The initial container process list
	// should contain the second container's init process
	buffers := newStdBuffers()
	ps := &libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"ps"},
		Env:    standardEnvironment,
		Stdout: buffers.Stdout,
	}
	err = container1.Run(ps)
	ok(t, err)
	waitProcess(ps, t)

	// Stop init processes one by one. Stop the second container should
	// not stop the first.
	_ = stdinW2.Close()
	waitProcess(init2, t)
	_ = stdinW1.Close()
	waitProcess(init1, t)

	out := strings.TrimSpace(buffers.Stdout.String())
	// output of ps inside the initial PID namespace should have
	// 1 line of header,
	// 2 lines of init processes,
	// 1 line of ps process
	if len(strings.Split(out, "\n")) != 4 {
		t.Errorf("unexpected running process, output %q", out)
	}
}

func TestInitJoinNetworkAndUser(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
		t.Skip("Test requires userns.")
	}
	if testing.Short() {
		return
	}

	// Execute a long-running container
	config1 := newTemplateConfig(t, &tParam{userns: true})
	container1, err := newContainer(t, config1)
	ok(t, err)
	defer destroyContainer(container1)

	stdinR1, stdinW1, err := os.Pipe()
	ok(t, err)
	init1 := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR1,
		Init:  true,
	}
	err = container1.Run(init1)
	_ = stdinR1.Close()
	defer stdinW1.Close() //nolint: errcheck
	ok(t, err)

	// get the state of the first container
	state1, err := container1.State()
	ok(t, err)
	netns1 := state1.NamespacePaths[configs.NEWNET]
	userns1 := state1.NamespacePaths[configs.NEWUSER]

	// Run a container inside the existing pidns but with different cgroups.
	config2 := newTemplateConfig(t, &tParam{userns: true})
	config2.Namespaces.Add(configs.NEWNET, netns1)
	config2.Namespaces.Add(configs.NEWUSER, userns1)
	config2.Cgroups.Path = "integration/test2"
	container2, err := newContainer(t, config2)
	ok(t, err)
	defer destroyContainer(container2)

	stdinR2, stdinW2, err := os.Pipe()
	ok(t, err)
	init2 := &libcontainer.Process{
		Cwd:   "/",
		Args:  []string{"cat"},
		Env:   standardEnvironment,
		Stdin: stdinR2,
		Init:  true,
	}
	err = container2.Run(init2)
	_ = stdinR2.Close()
	defer stdinW2.Close() //nolint: errcheck
	ok(t, err)

	// get the state of the second container
	state2, err := container2.State()
	ok(t, err)

	for _, ns := range []string{"net", "user"} {
		ns1, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", state1.InitProcessPid, ns))
		ok(t, err)
		ns2, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/%s", state2.InitProcessPid, ns))
		ok(t, err)
		if ns1 != ns2 {
			t.Errorf("%s(%s), wanted %s", ns, ns2, ns1)
		}
	}

	// check that namespaces are not the same
	if reflect.DeepEqual(state2.NamespacePaths, state1.NamespacePaths) {
		t.Errorf("Namespaces(%v), original %v", state2.NamespacePaths,
			state1.NamespacePaths)
	}
	// Stop init processes one by one. Stop the second container should
	// not stop the first.
	_ = stdinW2.Close()
	waitProcess(init2, t)
	_ = stdinW1.Close()
	waitProcess(init1, t)
}

func TestTmpfsCopyUp(t *testing.T) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, nil)
	config.Mounts = append(config.Mounts, &configs.Mount{
		Source:      "tmpfs",
		Destination: "/etc",
		Device:      "tmpfs",
		Extensions:  configs.EXT_COPYUP,
	})

	container, err := newContainer(t, config)
	ok(t, err)
	defer destroyContainer(container)

	var stdout bytes.Buffer
	pconfig := libcontainer.Process{
		Args:   []string{"ls", "/etc/passwd"},
		Env:    standardEnvironment,
		Stdin:  nil,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	// Wait for process
	waitProcess(&pconfig, t)

	outputLs := stdout.String()

	// Check that the ls output has /etc/passwd
	if !strings.Contains(outputLs, "/etc/passwd") {
		t.Fatalf("/etc/passwd not copied up as expected: %v", outputLs)
	}
}

func TestCGROUPPrivate(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
		t.Skip("Test requires cgroupns.")
	}
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/cgroup")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	config.Namespaces.Add(configs.NEWCGROUP, "")
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/cgroup")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual == l {
		t.Fatalf("cgroup link should be private to the container but equals host %q %q", actual, l)
	}
}

func TestCGROUPHost(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
		t.Skip("Test requires cgroupns.")
	}
	if testing.Short() {
		return
	}

	l, err := os.Readlink("/proc/1/ns/cgroup")
	ok(t, err)

	config := newTemplateConfig(t, nil)
	buffers := runContainerOk(t, config, "readlink", "/proc/self/ns/cgroup")

	if actual := strings.Trim(buffers.Stdout.String(), "\n"); actual != l {
		t.Fatalf("cgroup link not equal to host link %q %q", actual, l)
	}
}

func TestFdLeaks(t *testing.T) {
	testFdLeaks(t, false)
}

func TestFdLeaksSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	testFdLeaks(t, true)
}

func testFdLeaks(t *testing.T, systemd bool) {
	if testing.Short() {
		return
	}

	config := newTemplateConfig(t, &tParam{systemd: systemd})
	// Run a container once to exclude file descriptors that are only
	// opened once during the process lifetime by the library and are
	// never closed. Those are not considered leaks.
	//
	// Examples of this open-once file descriptors are:
	//  - /sys/fs/cgroup dirfd opened by prepareOpenat2 in libct/cgroups;
	//  - dbus connection opened by getConnection in libct/cgroups/systemd.
	_ = runContainerOk(t, config, "true")

	pfd, err := os.Open("/proc/self/fd")
	ok(t, err)
	defer pfd.Close()
	fds0, err := pfd.Readdirnames(0)
	ok(t, err)
	_, err = pfd.Seek(0, 0)
	ok(t, err)

	_ = runContainerOk(t, config, "true")

	fds1, err := pfd.Readdirnames(0)
	ok(t, err)

	if len(fds1) == len(fds0) {
		return
	}
	// Show the extra opened files.

	excludedPaths := []string{
		"anon_inode:bpf-prog", // FIXME: see https://github.com/opencontainers/runc/issues/2366#issuecomment-776411392
	}

	count := 0
next_fd:
	for _, fd1 := range fds1 {
		for _, fd0 := range fds0 {
			if fd0 == fd1 {
				continue next_fd
			}
		}
		dst, _ := os.Readlink("/proc/self/fd/" + fd1)
		for _, ex := range excludedPaths {
			if ex == dst {
				continue next_fd
			}
		}

		count++
		t.Logf("extra fd %s -> %s", fd1, dst)
	}
	if count > 0 {
		t.Fatalf("found %d extra fds after container.Run", count)
	}
}

// Test that a container using user namespaces is able to bind mount a folder
// that does not have permissions for group/others.
func TestBindMountAndUser(t *testing.T) {
	if _, err := os.Stat("/proc/self/ns/user"); errors.Is(err, os.ErrNotExist) {
		t.Skip("userns is unsupported")
	}

	if testing.Short() {
		return
	}

	temphost := t.TempDir()
	dirhost := filepath.Join(temphost, "inaccessible", "dir")

	err := os.MkdirAll(dirhost, 0o755)
	ok(t, err)

	err = os.WriteFile(filepath.Join(dirhost, "foo.txt"), []byte("Hello"), 0o755)
	ok(t, err)

	// Make this dir inaccessible to "group,others".
	err = os.Chmod(filepath.Join(temphost, "inaccessible"), 0o700)
	ok(t, err)

	config := newTemplateConfig(t, &tParam{
		userns: true,
	})

	// Set HostID to 1000 to avoid DAC_OVERRIDE bypassing the purpose of this test.
	config.UIDMappings[0].HostID = 1000
	config.GIDMappings[0].HostID = 1000

	// Set the owner of rootfs to the effective IDs in the host to avoid errors
	// while creating the folders to perform the mounts.
	err = os.Chown(config.Rootfs, 1000, 1000)
	ok(t, err)

	config.Mounts = append(config.Mounts, &configs.Mount{
		Source:      dirhost,
		Destination: "/tmp/mnt1cont",
		Device:      "bind",
		Flags:       unix.MS_BIND | unix.MS_REC,
	})

	container, err := newContainer(t, config)
	ok(t, err)
	defer container.Destroy() //nolint: errcheck

	var stdout bytes.Buffer

	pconfig := libcontainer.Process{
		Cwd:    "/",
		Args:   []string{"sh", "-c", "stat /tmp/mnt1cont/foo.txt"},
		Env:    standardEnvironment,
		Stdout: &stdout,
		Init:   true,
	}
	err = container.Run(&pconfig)
	ok(t, err)

	waitProcess(&pconfig, t)
}

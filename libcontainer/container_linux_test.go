package libcontainer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
)

type mockProcess struct {
	_pid    int
	started uint64
}

func (m *mockProcess) terminate() error {
	return nil
}

func (m *mockProcess) pid() int {
	return m._pid
}

func (m *mockProcess) startTime() (uint64, error) {
	return m.started, nil
}

func (m *mockProcess) start() error {
	return nil
}

func (m *mockProcess) wait() (*os.ProcessState, error) {
	return nil, nil
}

func (m *mockProcess) signal(_ os.Signal) error {
	return nil
}

func (m *mockProcess) externalDescriptors() []string {
	return []string{}
}

func (m *mockProcess) setExternalDescriptors(newFds []string) {
}

func (m *mockProcess) forwardChildLogs() chan error {
	return nil
}

// newFakeCgroupManager returns a real cgroup v2 manager operating on a
// temporary directory rather than on cgroupfs. This relies on cgroups.TestMode,
// which makes writes create the files being written to; files that are read
// have to be created by us.
//
// NOTE a test calling this can't use t.Parallel (because of cgroups.TestMode).
func newFakeCgroupManager(t *testing.T, config *cgroups.Cgroup) (cgroups.Manager, string) {
	t.Helper()
	origTestMode := cgroups.TestMode
	cgroups.TestMode = true
	t.Cleanup(func() { cgroups.TestMode = origTestMode })

	dir := t.TempDir()
	// Read by fs2 Set to figure out which controllers are available.
	if err := os.WriteFile(filepath.Join(dir, "cgroup.controllers"), []byte("cpu memory pids"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := fs2.NewManager(config, dir)
	if err != nil {
		t.Fatal(err)
	}
	return m, dir
}

func TestGetContainerStateAfterUpdate(t *testing.T) {
	pid := os.Getpid()
	stat, err := system.Stat(pid)
	if err != nil {
		t.Fatal(err)
	}

	cgConfig := &cgroups.Cgroup{
		Resources: &cgroups.Resources{
			Memory: 1024,
		},
	}
	cgManager, cgDir := newFakeCgroupManager(t, cgConfig)

	container := &Container{
		stateDir: t.TempDir(),
		id:       "myid",
		config: &configs.Config{
			Namespaces: []configs.Namespace{
				{Type: configs.NEWPID},
				{Type: configs.NEWNS},
				{Type: configs.NEWNET},
				{Type: configs.NEWUTS},
				{Type: configs.NEWIPC},
			},
			Cgroups: cgConfig,
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: stat.StartTime,
		},
		cgroupManager: cgManager,
	}
	container.state = &createdState{c: container}
	state, err := container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.InitProcessPid != pid {
		t.Fatalf("expected pid %d but received %d", pid, state.InitProcessPid)
	}
	if state.InitProcessStartTime != stat.StartTime {
		t.Fatalf("expected process start time %d but received %d", stat.StartTime, state.InitProcessStartTime)
	}
	if state.CgroupPaths[""] != cgDir {
		t.Fatalf("expected cgroup path %q but received %q", cgDir, state.CgroupPaths[""])
	}
	if state.Config.Cgroups.Resources.Memory != 1024 {
		t.Fatalf("expected Memory to be 1024 but received %d", state.Config.Cgroups.Resources.Memory)
	}

	// Set initProcessStartTime so we fake to be running
	container.initProcessStartTime = state.InitProcessStartTime
	container.state = &runningState{c: container}
	newConfig := container.Config()
	newConfig.Cgroups.Resources.Memory = 2048
	if err := container.Set(newConfig); err != nil {
		t.Fatal(err)
	}
	state, err = container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Config.Cgroups.Resources.Memory != 2048 {
		t.Fatalf("expected Memory to be 2048 but received %d", state.Config.Cgroups.Resources.Memory)
	}
	// Check the new value actually made it to the cgroup, not merely to state.json.
	data, err := os.ReadFile(filepath.Join(cgDir, "memory.max"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "2048" {
		t.Fatalf("expected memory.max to be 2048 but received %q", data)
	}
}

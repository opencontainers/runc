package libcontainer

import (
	"fmt"
	"os"
	"testing"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
)

type mockCgroupManager struct {
	pids    []int
	allPids []int
	paths   map[string]string
}

func (m *mockCgroupManager) GetPids() ([]int, error) {
	return m.pids, nil
}

func (m *mockCgroupManager) GetAllPids() ([]int, error) {
	return m.allPids, nil
}

func (m *mockCgroupManager) GetStats() (*cgroups.Stats, error) {
	return nil, nil
}

func (m *mockCgroupManager) Apply(pid int) error {
	return nil
}

func (m *mockCgroupManager) AddPid(_ string, _ int) error {
	return nil
}

func (m *mockCgroupManager) Set(_ *cgroups.Resources) error {
	return nil
}

func (m *mockCgroupManager) Destroy() error {
	return nil
}

func (m *mockCgroupManager) Exists() bool {
	_, err := os.Lstat(m.Path("devices"))
	return err == nil
}

func (m *mockCgroupManager) OOMKillCount() (uint64, error) {
	return 0, nil
}

func (m *mockCgroupManager) GetPaths() map[string]string {
	return m.paths
}

func (m *mockCgroupManager) Path(subsys string) string {
	return m.paths[subsys]
}

func (m *mockCgroupManager) Freeze(_ cgroups.FreezerState) error {
	return nil
}

func (m *mockCgroupManager) GetCgroups() (*cgroups.Cgroup, error) {
	return nil, nil
}

func (m *mockCgroupManager) GetFreezerState() (cgroups.FreezerState, error) {
	return cgroups.Thawed, nil
}

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

func TestGetContainerPids(t *testing.T) {
	pid := 1
	stat, err := system.Stat(pid)
	if err != nil {
		t.Fatalf("can't stat pid %d, got %v", pid, err)
	}
	container := &Container{
		id:     "myid",
		config: &configs.Config{},
		cgroupManager: &mockCgroupManager{
			allPids: []int{1, 2, 3},
			paths: map[string]string{
				"device": "/proc/self/cgroups",
			},
		},
		initProcess: &mockProcess{
			_pid:    1,
			started: 10,
		},
		initProcessStartTime: stat.StartTime,
	}
	container.state = &runningState{c: container}
	pids, err := container.Processes()
	if err != nil {
		t.Fatal(err)
	}
	for i, expected := range []int{1, 2, 3} {
		if pids[i] != expected {
			t.Fatalf("expected pid %d but received %d", expected, pids[i])
		}
	}
}

func TestGetContainerState(t *testing.T) {
	var (
		pid                 = os.Getpid()
		expectedMemoryPath  = "/sys/fs/cgroup/memory/myid"
		expectedNetworkPath = fmt.Sprintf("/proc/%d/ns/net", pid)
	)
	container := &Container{
		id: "myid",
		config: &configs.Config{
			Namespaces: []configs.Namespace{
				{Type: configs.NEWPID},
				{Type: configs.NEWNS},
				{Type: configs.NEWNET, Path: expectedNetworkPath},
				{Type: configs.NEWUTS},
				// emulate host for IPC
				//{Type: configs.NEWIPC},
				{Type: configs.NEWCGROUP},
			},
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: 10,
		},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			paths: map[string]string{
				"memory": expectedMemoryPath,
			},
		},
	}
	container.state = &createdState{c: container}
	state, err := container.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.InitProcessPid != pid {
		t.Fatalf("expected pid %d but received %d", pid, state.InitProcessPid)
	}
	if state.InitProcessStartTime != 10 {
		t.Fatalf("expected process start time 10 but received %d", state.InitProcessStartTime)
	}
	paths := state.CgroupPaths
	if paths == nil {
		t.Fatal("cgroup paths should not be nil")
	}
	if memPath := paths["memory"]; memPath != expectedMemoryPath {
		t.Fatalf("expected memory path %q but received %q", expectedMemoryPath, memPath)
	}
	for _, ns := range container.config.Namespaces {
		path := state.NamespacePaths[ns.Type]
		if path == "" {
			t.Fatalf("expected non nil namespace path for %s", ns.Type)
		}

		var expected string
		if ns.Type == configs.NEWNET {
			expected = expectedNetworkPath
		} else {
			expected = ns.GetPath(pid)
		}
		if expected != path {
			t.Fatalf("expected path %q but received %q", expected, path)
		}
	}
}

func TestGetContainerStateAfterUpdate(t *testing.T) {
	pid := os.Getpid()
	stat, err := system.Stat(pid)
	if err != nil {
		t.Fatal(err)
	}

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
			Cgroups: &cgroups.Cgroup{
				Resources: &cgroups.Resources{
					Memory: 1024,
				},
			},
		},
		initProcess: &mockProcess{
			_pid:    pid,
			started: stat.StartTime,
		},
		cgroupManager: &mockCgroupManager{},
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
	if state.Config.Cgroups.Resources.Memory != 1024 {
		t.Fatalf("expected Memory to be 1024 but received %q", state.Config.Cgroups.Memory)
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
		t.Fatalf("expected Memory to be 2048 but received %q", state.Config.Cgroups.Memory)
	}
}

// +build linux

package libcontainer

import (
	"testing"

	"github.com/docker/libcontainer/cgroups"
)

type mockCgroupManager struct {
	pids  []int
	stats *cgroups.Stats
}

func (m *mockCgroupManager) GetPids(config *cgroups.Cgroup) ([]int, error) {
	return m.pids, nil
}

func (m *mockCgroupManager) GetStats(config *cgroups.Cgroup) (*cgroups.Stats, error) {
	return m.stats, nil
}

func TestGetContainerPids(t *testing.T) {
	container := &linuxContainer{
		id:            "myid",
		config:        &Config{},
		cgroupManager: &mockCgroupManager{pids: []int{1, 2, 3}},
	}

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

func TestGetContainerStats(t *testing.T) {
	container := &linuxContainer{
		id:     "myid",
		config: &Config{},
		cgroupManager: &mockCgroupManager{
			pids: []int{1, 2, 3},
			stats: &cgroups.Stats{
				MemoryStats: cgroups.MemoryStats{
					Usage: 1024,
				},
			},
		},
		state: &State{},
	}

	stats, err := container.Stats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.CgroupStats == nil {
		t.Fatal("cgroup stats are nil")
	}
	if stats.CgroupStats.MemoryStats.Usage != 1024 {
		t.Fatalf("expected memory usage 1024 but recevied %d", stats.CgroupStats.MemoryStats.Usage)
	}
}

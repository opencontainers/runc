package fs2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const exampleMemoryStatData = `anon 790425600
file 6502666240
kernel_stack 7012352
pagetables 8867840
percpu 2445520
sock 40960
shmem 6721536
file_mapped 656187392
file_dirty 1122304
file_writeback 0
swapcached 10
anon_thp 438304768
file_thp 0
shmem_thp 0
inactive_anon 892223488
active_anon 2973696
inactive_file 5307346944
active_file 1179316224
unevictable 31477760
slab_reclaimable 348866240
slab_unreclaimable 10099808
slab 358966048
workingset_refault_anon 0
workingset_refault_file 0
workingset_activate_anon 0
workingset_activate_file 0
workingset_restore_anon 0
workingset_restore_file 0
workingset_nodereclaim 0
pgfault 103216687
pgmajfault 6879
pgrefill 0
pgscan 0
pgsteal 0
pgactivate 1110217
pgdeactivate 292
pglazyfree 267
pglazyfreed 0
thp_fault_alloc 57411
thp_collapse_alloc 443`

func TestStatMemoryPodCgroupNotFound(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true
	fakeCgroupDir := t.TempDir()

	// only write memory.stat to ensure pod cgroup usage
	// still reads memory.current.
	statPath := filepath.Join(fakeCgroupDir, "memory.stat")
	if err := os.WriteFile(statPath, []byte(exampleMemoryStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	gotStats := cgroups.NewStats()

	// use a fake root path to mismatch the file we wrote.
	// this triggers the non-root path which should fail to find memory.current.
	err := statMemory(fakeCgroupDir, gotStats)
	if err == nil {
		t.Errorf("expected error when statting memory for cgroupv2 root, but was nil")
	}

	if !strings.Contains(err.Error(), "memory.current: no such file or directory") {
		t.Errorf("expected error to contain 'memory.current: no such file or directory', but was %s", err.Error())
	}
}

func TestStatMemoryPodCgroup(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true
	fakeCgroupDir := t.TempDir()

	statPath := filepath.Join(fakeCgroupDir, "memory.stat")
	if err := os.WriteFile(statPath, []byte(exampleMemoryStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.current"), []byte("123456789"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.max"), []byte("999999999"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(fakeCgroupDir, "memory.peak"), []byte("987654321"), 0o644); err != nil {
		t.Fatal(err)
	}

	gotStats := cgroups.NewStats()

	// use a fake root path to trigger the pod cgroup lookup.
	err := statMemory(fakeCgroupDir, gotStats)
	if err != nil {
		t.Errorf("expected no error when statting memory for cgroupv2 root, but got %#+v", err)
	}

	// result should be "memory.current"
	var expectedUsageBytes uint64 = 123456789
	if gotStats.MemoryStats.Usage.Usage != expectedUsageBytes {
		t.Errorf("parsed cgroupv2 memory.stat doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.MemoryStats.Usage.Usage, expectedUsageBytes)
	}

	// result should be "memory.max"
	var expectedLimitBytes uint64 = 999999999
	if gotStats.MemoryStats.Usage.Limit != expectedLimitBytes {
		t.Errorf("parsed cgroupv2 memory.stat doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.MemoryStats.Usage.Limit, expectedLimitBytes)
	}

	// result should be "memory.peak"
	var expectedMaxUsageBytes uint64 = 987654321
	if gotStats.MemoryStats.Usage.MaxUsage != expectedMaxUsageBytes {
		t.Errorf("parsed cgroupv2 memory.stat doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.MemoryStats.Usage.MaxUsage, expectedMaxUsageBytes)
	}
}

func TestRootStatsFromMeminfo(t *testing.T) {
	stats := &cgroups.Stats{
		MemoryStats: cgroups.MemoryStats{
			Stats: map[string]uint64{
				"anon": 790425600,
				"file": 6502666240,
			},
		},
	}

	if err := rootStatsFromMeminfo(stats); err != nil {
		t.Fatal(err)
	}

	// result is anon + file
	var expectedUsageBytes uint64 = 7293091840
	if stats.MemoryStats.Usage.Usage != expectedUsageBytes {
		t.Errorf("parsed cgroupv2 memory.stat doesn't match expected result: \ngot %d\nexpected %d\n", stats.MemoryStats.Usage.Usage, expectedUsageBytes)
	}

	// swap is adjusted to mem+swap
	if stats.MemoryStats.SwapUsage.Usage < stats.MemoryStats.Usage.Usage {
		t.Errorf("swap usage %d should be at least mem usage %d", stats.MemoryStats.SwapUsage.Usage, stats.MemoryStats.Usage.Usage)
	}
	if stats.MemoryStats.SwapUsage.Limit < stats.MemoryStats.Usage.Limit {
		t.Errorf("swap limit %d should be at least mem limit %d", stats.MemoryStats.SwapUsage.Limit, stats.MemoryStats.Usage.Limit)
	}
}

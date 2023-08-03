package fs2

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	memoryStatContents = `anon 6121017344
file 26672709632
kernel 1461346304
kernel_stack 25952256
pagetables 82386944
sec_pagetables 0
percpu 294409248
sock 16384
vmalloc 2097152
shmem 6836224
zswap 0
zswapped 0
file_mapped 993832960
file_dirty 4272128
file_writeback 0
swapcached 0
anon_thp 62914560
file_thp 0
shmem_thp 0
inactive_anon 17866752
active_anon 6109827072
inactive_file 20126994432
active_file 6537940992
unevictable 28418048
slab_reclaimable 1008383568
slab_unreclaimable 35473416
slab 1043856984
workingset_refault_anon 0
workingset_refault_file 49
workingset_activate_anon 0
workingset_activate_file 49
workingset_restore_anon 0
workingset_restore_file 10
workingset_nodereclaim 0
pgscan 262336
pgsteal 262306
pgscan_kswapd 0
pgscan_direct 262336
pgscan_khugepaged 0
pgsteal_kswapd 0
pgsteal_direct 262306
pgsteal_khugepaged 0
pgfault 0
pgmajfault 0
pgrefill 1
pgactivate 0
pgdeactivate 0
pglazyfree 3621
pglazyfreed 0
zswpin 0
zswpout 0
thp_fault_alloc 0
thp_collapse_alloc 0`
	memoryUsageContents        = "2048\n"
	memoryMaxUsageContents     = "4096\n"
	memoryLimitContents        = "8192\n"
	memoryUseHierarchyContents = "1\n"
	memoryNUMAStatContents     = `anon N0=139022336 N1=2760704 N2=139022336 N3=2760704
file N0=449581056 N1=4312 N2=449581056 N3=4312
kernel_stack N0=3670016 N1=43134 N2=3670016 N3=43134
pagetables N0=4116480 N1=43214 N2=4116480 N3=43214
sec_pagetables N0=0 N1=53 N2=0 N3=53
shmem N0=0 N1=0 N2=0 N3=0
file_mapped N0=55029760 N1=0 N2=55029760 N3=0
file_dirty N0=0 N1=0 N2=0 N3=0
file_writeback N0=0 N1=0 N2=0 N3=0
swapcached N0=0 N1=0 N2=0 N3=0
anon_thp N0=0 N1=0 N2=0 N3=0
file_thp N0=0 N1=0 N2=0 N3=0
shmem_thp N0=0 N1=0 N2=0 N3=0
inactive_anon N0=138956800 N1=2752512 N2=138956800 N3=2752512
active_anon N0=65536 N1=8192 N2=65536 N3=8192
inactive_file N0=14770176 N1=0 N2=14770176 N3=0
active_file N0=434810880 N1=0 N2=434810880 N3=0
unevictable N0=34215 N1=2512 N2=34215 N3=2512
slab_reclaimable N0=2358224 N1=11088 N2=2358224 N3=11088
slab_unreclaimable N0=2672352 N1=544144 N2=2672352 N3=544144
`
	// Some custom kernels has extra fields that should be ignored.
	memoryNUMAStatExtraContents = `workingset_refault_anon N0=0 N1=0 N2=0 N3=0
workingset_refault_file N0=0 N1=0 N2=0 N3=0
workingset_activate_anon N0=0 N1=0 N2=0 N3=0
workingset_activate_file N0=0 N1=0 N2=0 N3=0
workingset_restore_anon N0=0 N1=0 N2=0 N3=0
workingset_restore_file N0=0 N1=0 N2=0 N3=0
workingset_nodereclaim N0=0 N1=0 N2=0 N3=0
`
)

func TestMemorySetMemoryV2(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	lowPath := filepath.Join(fakeCgroupDir, "memory.low")
	maxPath := filepath.Join(fakeCgroupDir, "memory.max")

	const (
		memoryBefore      = 314572800 // 300M
		memoryAfter       = 524288000 // 500M
		reservationBefore = 209715200 // 200M
		reservationAfter  = 314572800 // 300M
	)

	if err := os.WriteFile(maxPath, []byte(strconv.Itoa(memoryBefore)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lowPath, []byte(strconv.Itoa(reservationBefore)), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &configs.Resources{
		Memory:            memoryAfter,
		MemoryReservation: reservationAfter,
	}
	if err := setMemory(fakeCgroupDir, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(fakeCgroupDir, "memory.max")
	if err != nil {
		t.Error(err)
	}
	if value != memoryAfter {
		t.Fatalf("Got the wrong value %d, set memory.low failed.", value)
	}

	if value, err = fscommon.GetCgroupParamUint(fakeCgroupDir, "memory.low"); err != nil {
		t.Fatal(err)
	}
	if value != reservationAfter {
		t.Fatal("Got the wrong value, set memory.soft_limit_in_bytes failed.")
	}
}

func TestMemorySetMemoryswapV2(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	maxPath := filepath.Join(fakeCgroupDir, "memory.max")
	lowPath := filepath.Join(fakeCgroupDir, "memory.low")
	swapMaxPath := filepath.Join(fakeCgroupDir, "memory.swap.max")

	const (
		memoryswapBefore = 314572800 // 300M
		memoryswapAfter  = 524288000 // 500M
	)
	if err := os.WriteFile(maxPath, []byte(strconv.Itoa(1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lowPath, []byte(strconv.Itoa(1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(swapMaxPath, []byte(strconv.Itoa(memoryswapBefore)), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &configs.Resources{
		Memory:            1,
		MemoryReservation: 1,
		MemorySwap:        memoryswapAfter,
	}
	if err := setMemory(fakeCgroupDir, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(fakeCgroupDir, "memory.swap.max")
	if err != nil {
		t.Error(err)
	}
	if value != memoryswapAfter-1 {
		t.Fatalf("Got the wrong value %d, set memory.swap.max failed.", value)
	}
}

func TestMemoryStats(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	statPath := filepath.Join(fakeCgroupDir, "memory.stat")
	maxPath := filepath.Join(fakeCgroupDir, "memory.max")
	lowPath := filepath.Join(fakeCgroupDir, "memory.low")
	currentPath := filepath.Join(fakeCgroupDir, "memory.current")
	swapMaxPath := filepath.Join(fakeCgroupDir, "memory.swap.max")
	numaStatPath := filepath.Join(fakeCgroupDir, "memory.numa_stat")
	fakesub1CgroupDir := filepath.Join(fakeCgroupDir, "test1")
	subnuma1StatPath := filepath.Join(fakesub1CgroupDir, "memory.numa_stat")
	fakesub2CgroupDir := filepath.Join(fakeCgroupDir, "test2")
	subnuma2StatPath := filepath.Join(fakesub2CgroupDir, "memory.numa_stat")

	if err := os.WriteFile(statPath, []byte(memoryStatContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(maxPath, []byte(memoryMaxUsageContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lowPath, []byte(memoryLimitContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(currentPath, []byte(memoryUsageContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(swapMaxPath, []byte(memoryMaxUsageContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(numaStatPath, []byte(memoryNUMAStatContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakesub1CgroupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subnuma1StatPath, []byte(memoryNUMAStatContents), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fakesub2CgroupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subnuma2StatPath, []byte(memoryNUMAStatContents), 0o644); err != nil {
		t.Fatal(err)
	}

	actualStats := *cgroups.NewStats()
	err := statMemory(fakeCgroupDir, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.MemoryStats{
		Cache:        26672709632,
		Usage:        cgroups.MemoryData{Usage: 2048, MaxUsage: 0, Failcnt: 0, Limit: 4096},
		SwapUsage:    cgroups.MemoryData{Usage: 2048, MaxUsage: 0, Failcnt: 0, Limit: 4096},
		Stats:        map[string]uint64{"file": 26672709632, "anon": 6121017344},
		UseHierarchy: true,
		PageUsageByNUMA: cgroups.PageUsageByNUMA{
			PageUsageByNUMAInner: cgroups.PageUsageByNUMAInner{
				Total:       cgroups.PageStats{Total: 0x467f21b0, Nodes: map[uint8]uint64{0x0: 0x23156000, 0x1: 0x2a30d8, 0x2: 0x23156000, 0x3: 0x2a30d8}},
				File:        cgroups.PageStats{Total: 0x359841b0, Nodes: map[uint8]uint64{0x0: 0x1acc1000, 0x1: 0x10d8, 0x2: 0x1acc1000, 0x3: 0x10d8}},
				Anon:        cgroups.PageStats{Total: 0x10e6e000, Nodes: map[uint8]uint64{0x0: 0x8495000, 0x1: 0x2a2000, 0x2: 0x8495000, 0x3: 0x2a2000}},
				Unevictable: cgroups.PageStats{Total: 0x11eee, Nodes: map[uint8]uint64{0x0: 0x85a7, 0x1: 0x9d0, 0x2: 0x85a7, 0x3: 0x9d0}},
			},
			Hierarchical: cgroups.PageUsageByNUMAInner{
				Total:       cgroups.PageStats{Total: 0x8cfe4360, Nodes: map[uint8]uint64{0x0: 0x462ac000, 0x1: 0x5461b0, 0x2: 0x462ac000, 0x3: 0x5461b0}},
				File:        cgroups.PageStats{Total: 0x6b308360, Nodes: map[uint8]uint64{0x0: 0x35982000, 0x1: 0x21b0, 0x2: 0x35982000, 0x3: 0x21b0}},
				Anon:        cgroups.PageStats{Total: 0x21cdc000, Nodes: map[uint8]uint64{0x0: 0x1092a000, 0x1: 0x544000, 0x2: 0x1092a000, 0x3: 0x544000}},
				Unevictable: cgroups.PageStats{Total: 0x23ddc, Nodes: map[uint8]uint64{0x0: 0x10b4e, 0x1: 0x13a0, 0x2: 0x10b4e, 0x3: 0x13a0}},
			},
		},
	}
	expectMemoryStatEquals(t, expectedStats, actualStats.MemoryStats)
}

func TestNoHierarchicalNumaStat(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	numastatPath := filepath.Join(fakeCgroupDir, "memory.numa_stat")

	if err := os.WriteFile(numastatPath, []byte(memoryNUMAStatContents+memoryNUMAStatExtraContents), 0o644); err != nil {
		t.Fatal(err)
	}

	actualStats, err := getPageUsageByNUMAV2(fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}
	pageUsageByNUMA := cgroups.PageUsageByNUMA{
		PageUsageByNUMAInner: cgroups.PageUsageByNUMAInner{
			Total:       cgroups.PageStats{Total: 1182736816, Nodes: map[uint8]uint64{0: 588603392, 1: 2765016, 2: 588603392, 3: 2765016}},
			File:        cgroups.PageStats{Total: 899170736, Nodes: map[uint8]uint64{0: 449581056, 1: 4312, 2: 449581056, 3: 4312}},
			Anon:        cgroups.PageStats{Total: 283566080, Nodes: map[uint8]uint64{0: 139022336, 1: 2760704, 2: 139022336, 3: 2760704}},
			Unevictable: cgroups.PageStats{Total: 73454, Nodes: map[uint8]uint64{0: 34215, 1: 2512, 2: 34215, 3: 2512}},
		},
		Hierarchical: cgroups.PageUsageByNUMAInner{},
	}
	expectPageUsageByNUMAEquals(t, pageUsageByNUMA, actualStats)
}

func TestBadNumaStat(t *testing.T) {
	memoryNUMAStatBadContents := []struct {
		desc, contents string
	}{
		{
			desc: "Nx where x is not a number",
			contents: `anon N0=44611
file=44428 Nx=0
`,
		}, {
			desc:     "Nx where x > 255",
			contents: `anon N333=444`,
		}, {
			desc:     "Nx argument missing",
			contents: `anon N0=123 N1=`,
		}, {
			desc:     "Nx argument is not a number",
			contents: `anon N0=123 N1=a`,
		}, {
			desc:     "Missing = after Nx",
			contents: `anon N0=123 N1`,
		}, {
			desc: "No Nx at non-first position",
			contents: `anon N0=32631
file N0=32614
unevictable N0=12 badone
`,
		},
	}
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	numastatPath := filepath.Join(fakeCgroupDir, "memory.numa_stat")

	for _, c := range memoryNUMAStatBadContents {
		if err := os.WriteFile(numastatPath, []byte(c.contents), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := getPageUsageByNUMAV2(fakeCgroupDir)

		if err == nil {
			t.Errorf("case %q: expected error, got nil", c.desc)
		}
	}
}

func TestWithoutNumaStat(t *testing.T) {
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()

	actualStats, err := getPageUsageByNUMAV2(fakeCgroupDir)
	if err != nil {
		t.Fatal(err)
	}
	expectPageUsageByNUMAEquals(t, cgroups.PageUsageByNUMA{}, actualStats)
}

package fs

import (
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	memoryStatContents = `cache 512
rss 1024`
	memoryUsageContents        = "2048\n"
	memoryMaxUsageContents     = "4096\n"
	memoryFailcnt              = "100\n"
	memoryLimitContents        = "8192\n"
	memoryUseHierarchyContents = "1\n"
	memoryNUMAStatContents     = `total=44611 N0=32631 N1=7501 N2=1982 N3=2497
file=44428 N0=32614 N1=7335 N2=1982 N3=2497
anon=183 N0=17 N1=166 N2=0 N3=0
unevictable=0 N0=0 N1=0 N2=0 N3=0
hierarchical_total=768133 N0=509113 N1=138887 N2=20464 N3=99669
hierarchical_file=722017 N0=496516 N1=119997 N2=20181 N3=85323
hierarchical_anon=46096 N0=12597 N1=18890 N2=283 N3=14326
hierarchical_unevictable=20 N0=0 N1=0 N2=0 N3=20
`
	memoryNUMAStatNoHierarchyContents = `total=44611 N0=32631 N1=7501 N2=1982 N3=2497
file=44428 N0=32614 N1=7335 N2=1982 N3=2497
anon=183 N0=17 N1=166 N2=0 N3=0
unevictable=0 N0=0 N1=0 N2=0 N3=0
`
	// Some custom kernels has extra fields that should be ignored
	memoryNUMAStatExtraContents = `numa_locality 0 0 0 0 0 0 0 0 0 0
numa_exectime 0
whatever=100 N0=0
`
)

func TestMemorySetMemory(t *testing.T) {
	path := tempDir(t, "memory")

	const (
		memoryBefore      = 314572800 // 300M
		memoryAfter       = 524288000 // 500M
		reservationBefore = 209715200 // 200M
		reservationAfter  = 314572800 // 300M
	)

	writeFileContents(t, path, map[string]string{
		"memory.limit_in_bytes":      strconv.Itoa(memoryBefore),
		"memory.soft_limit_in_bytes": strconv.Itoa(reservationBefore),
	})

	r := &configs.Resources{
		Memory:            memoryAfter,
		MemoryReservation: reservationAfter,
	}
	memory := &MemoryGroup{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryAfter {
		t.Fatal("Got the wrong value, set memory.limit_in_bytes failed.")
	}

	value, err = fscommon.GetCgroupParamUint(path, "memory.soft_limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != reservationAfter {
		t.Fatal("Got the wrong value, set memory.soft_limit_in_bytes failed.")
	}
}

func TestMemorySetMemoryswap(t *testing.T) {
	path := tempDir(t, "memory")

	const (
		memoryswapBefore = 314572800 // 300M
		memoryswapAfter  = 524288000 // 500M
	)

	writeFileContents(t, path, map[string]string{
		"memory.memsw.limit_in_bytes": strconv.Itoa(memoryswapBefore),
	})

	r := &configs.Resources{
		MemorySwap: memoryswapAfter,
	}
	memory := &MemoryGroup{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.memsw.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryswapAfter {
		t.Fatal("Got the wrong value, set memory.memsw.limit_in_bytes failed.")
	}
}

func TestMemorySetMemoryLargerThanSwap(t *testing.T) {
	path := tempDir(t, "memory")

	const (
		memoryBefore     = 314572800 // 300M
		memoryswapBefore = 524288000 // 500M
		memoryAfter      = 629145600 // 600M
		memoryswapAfter  = 838860800 // 800M
	)

	writeFileContents(t, path, map[string]string{
		"memory.limit_in_bytes":       strconv.Itoa(memoryBefore),
		"memory.memsw.limit_in_bytes": strconv.Itoa(memoryswapBefore),
		// Set will call getMemoryData when memory and swap memory are
		// both set, fake these fields so we don't get error.
		"memory.usage_in_bytes":     "0",
		"memory.max_usage_in_bytes": "0",
		"memory.failcnt":            "0",
	})

	r := &configs.Resources{
		Memory:     memoryAfter,
		MemorySwap: memoryswapAfter,
	}
	memory := &MemoryGroup{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryAfter {
		t.Fatal("Got the wrong value, set memory.limit_in_bytes failed.")
	}

	value, err = fscommon.GetCgroupParamUint(path, "memory.memsw.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryswapAfter {
		t.Fatal("Got the wrong value, set memory.memsw.limit_in_bytes failed.")
	}
}

func TestMemorySetSwapSmallerThanMemory(t *testing.T) {
	path := tempDir(t, "memory")

	const (
		memoryBefore     = 629145600 // 600M
		memoryswapBefore = 838860800 // 800M
		memoryAfter      = 314572800 // 300M
		memoryswapAfter  = 524288000 // 500M
	)

	writeFileContents(t, path, map[string]string{
		"memory.limit_in_bytes":       strconv.Itoa(memoryBefore),
		"memory.memsw.limit_in_bytes": strconv.Itoa(memoryswapBefore),
	})

	r := &configs.Resources{
		Memory:     memoryAfter,
		MemorySwap: memoryswapAfter,
	}
	memory := &MemoryGroup{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryAfter {
		t.Fatalf("Got the wrong value (%d != %d), set memory.limit_in_bytes failed", value, memoryAfter)
	}

	value, err = fscommon.GetCgroupParamUint(path, "memory.memsw.limit_in_bytes")
	if err != nil {
		t.Fatal(err)
	}
	if value != memoryswapAfter {
		t.Fatalf("Got the wrong value (%d != %d), set memory.memsw.limit_in_bytes failed", value, memoryswapAfter)
	}
}

func TestMemorySetMemorySwappinessDefault(t *testing.T) {
	path := tempDir(t, "memory")

	swappinessBefore := 60 // default is 60
	swappinessAfter := uint64(0)

	writeFileContents(t, path, map[string]string{
		"memory.swappiness": strconv.Itoa(swappinessBefore),
	})

	r := &configs.Resources{
		MemorySwappiness: &swappinessAfter,
	}
	memory := &MemoryGroup{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.swappiness")
	if err != nil {
		t.Fatal(err)
	}
	if value != swappinessAfter {
		t.Fatalf("Got the wrong value (%d), set memory.swappiness = %d failed.", value, swappinessAfter)
	}
}

func TestMemoryStats(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":                     memoryStatContents,
		"memory.usage_in_bytes":           memoryUsageContents,
		"memory.limit_in_bytes":           memoryLimitContents,
		"memory.max_usage_in_bytes":       memoryMaxUsageContents,
		"memory.failcnt":                  memoryFailcnt,
		"memory.memsw.usage_in_bytes":     memoryUsageContents,
		"memory.memsw.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.memsw.failcnt":            memoryFailcnt,
		"memory.memsw.limit_in_bytes":     memoryLimitContents,
		"memory.kmem.usage_in_bytes":      memoryUsageContents,
		"memory.kmem.max_usage_in_bytes":  memoryMaxUsageContents,
		"memory.kmem.failcnt":             memoryFailcnt,
		"memory.kmem.limit_in_bytes":      memoryLimitContents,
		"memory.use_hierarchy":            memoryUseHierarchyContents,
		"memory.numa_stat":                memoryNUMAStatContents + memoryNUMAStatExtraContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.MemoryStats{
		Cache:         512,
		Usage:         cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192},
		SwapUsage:     cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192},
		SwapOnlyUsage: cgroups.MemoryData{Usage: 0, MaxUsage: 0, Failcnt: 0, Limit: 0},
		KernelUsage:   cgroups.MemoryData{Usage: 2048, MaxUsage: 4096, Failcnt: 100, Limit: 8192},
		Stats:         map[string]uint64{"cache": 512, "rss": 1024},
		UseHierarchy:  true,
		PageUsageByNUMA: cgroups.PageUsageByNUMA{
			PageUsageByNUMAInner: cgroups.PageUsageByNUMAInner{
				Total:       cgroups.PageStats{Total: 44611, Nodes: map[uint8]uint64{0: 32631, 1: 7501, 2: 1982, 3: 2497}},
				File:        cgroups.PageStats{Total: 44428, Nodes: map[uint8]uint64{0: 32614, 1: 7335, 2: 1982, 3: 2497}},
				Anon:        cgroups.PageStats{Total: 183, Nodes: map[uint8]uint64{0: 17, 1: 166, 2: 0, 3: 0}},
				Unevictable: cgroups.PageStats{Total: 0, Nodes: map[uint8]uint64{0: 0, 1: 0, 2: 0, 3: 0}},
			},
			Hierarchical: cgroups.PageUsageByNUMAInner{
				Total:       cgroups.PageStats{Total: 768133, Nodes: map[uint8]uint64{0: 509113, 1: 138887, 2: 20464, 3: 99669}},
				File:        cgroups.PageStats{Total: 722017, Nodes: map[uint8]uint64{0: 496516, 1: 119997, 2: 20181, 3: 85323}},
				Anon:        cgroups.PageStats{Total: 46096, Nodes: map[uint8]uint64{0: 12597, 1: 18890, 2: 283, 3: 14326}},
				Unevictable: cgroups.PageStats{Total: 20, Nodes: map[uint8]uint64{0: 0, 1: 0, 2: 0, 3: 20}},
			},
		},
	}
	expectMemoryStatEquals(t, expectedStats, actualStats.MemoryStats)
}

func TestMemoryStatsNoStatFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.usage_in_bytes":     memoryUsageContents,
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.limit_in_bytes":     memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMemoryStatsNoUsageFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.limit_in_bytes":     memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsNoMaxUsageFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":           memoryStatContents,
		"memory.usage_in_bytes": memoryUsageContents,
		"memory.limit_in_bytes": memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsNoLimitInBytesFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.usage_in_bytes":     memoryUsageContents,
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsBadStatFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               "rss rss",
		"memory.usage_in_bytes":     memoryUsageContents,
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.limit_in_bytes":     memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsBadUsageFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.usage_in_bytes":     "bad",
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.limit_in_bytes":     memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsBadMaxUsageFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.usage_in_bytes":     memoryUsageContents,
		"memory.max_usage_in_bytes": "bad",
		"memory.limit_in_bytes":     memoryLimitContents,
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemoryStatsBadLimitInBytesFile(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.stat":               memoryStatContents,
		"memory.usage_in_bytes":     memoryUsageContents,
		"memory.max_usage_in_bytes": memoryMaxUsageContents,
		"memory.limit_in_bytes":     "bad",
	})

	memory := &MemoryGroup{}
	actualStats := *cgroups.NewStats()
	err := memory.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestMemorySetOomControl(t *testing.T) {
	path := tempDir(t, "memory")

	const (
		oomKillDisable = 1 // disable oom killer, default is 0
	)

	writeFileContents(t, path, map[string]string{
		"memory.oom_control": strconv.Itoa(oomKillDisable),
	})

	memory := &MemoryGroup{}
	r := &configs.Resources{}
	if err := memory.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "memory.oom_control")
	if err != nil {
		t.Fatal(err)
	}
	if value != oomKillDisable {
		t.Fatalf("Got the wrong value, set memory.oom_control failed.")
	}
}

func TestNoHierarchicalNumaStat(t *testing.T) {
	path := tempDir(t, "memory")
	writeFileContents(t, path, map[string]string{
		"memory.numa_stat": memoryNUMAStatNoHierarchyContents + memoryNUMAStatExtraContents,
	})

	actualStats, err := getPageUsageByNUMA(path)
	if err != nil {
		t.Fatal(err)
	}
	pageUsageByNUMA := cgroups.PageUsageByNUMA{
		PageUsageByNUMAInner: cgroups.PageUsageByNUMAInner{
			Total:       cgroups.PageStats{Total: 44611, Nodes: map[uint8]uint64{0: 32631, 1: 7501, 2: 1982, 3: 2497}},
			File:        cgroups.PageStats{Total: 44428, Nodes: map[uint8]uint64{0: 32614, 1: 7335, 2: 1982, 3: 2497}},
			Anon:        cgroups.PageStats{Total: 183, Nodes: map[uint8]uint64{0: 17, 1: 166, 2: 0, 3: 0}},
			Unevictable: cgroups.PageStats{Total: 0, Nodes: map[uint8]uint64{0: 0, 1: 0, 2: 0, 3: 0}},
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
			contents: `total=44611 N0=44611,
file=44428 Nx=0
`,
		}, {
			desc:     "Nx where x > 255",
			contents: `total=44611 N333=444`,
		}, {
			desc:     "Nx argument missing",
			contents: `total=44611 N0=123 N1=`,
		}, {
			desc:     "Nx argument is not a number",
			contents: `total=44611 N0=123 N1=a`,
		}, {
			desc:     "Missing = after Nx",
			contents: `total=44611 N0=123 N1`,
		}, {
			desc: "No Nx at non-first position",
			contents: `total=44611 N0=32631
file=44428 N0=32614
anon=183 N0=12 badone
`,
		},
	}
	path := tempDir(t, "memory")
	for _, c := range memoryNUMAStatBadContents {
		writeFileContents(t, path, map[string]string{
			"memory.numa_stat": c.contents,
		})

		_, err := getPageUsageByNUMA(path)
		if err == nil {
			t.Errorf("case %q: expected error, got nil", c.desc)
		}
	}
}

func TestWithoutNumaStat(t *testing.T) {
	path := tempDir(t, "memory")

	actualStats, err := getPageUsageByNUMA(path)
	if err != nil {
		t.Fatal(err)
	}
	expectPageUsageByNUMAEquals(t, cgroups.PageUsageByNUMA{}, actualStats)
}

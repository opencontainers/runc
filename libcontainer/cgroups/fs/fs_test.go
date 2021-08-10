package fs

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func BenchmarkGetStats(b *testing.B) {
	if cgroups.IsCgroup2UnifiedMode() {
		b.Skip("cgroup v2 is not supported")
	}

	// Unset TestMode as we work with real cgroupfs here,
	// and we want OpenFile to perform the fstype check.
	cgroups.TestMode = false
	defer func() {
		cgroups.TestMode = true
	}()

	cg := &configs.Cgroup{
		Path:      "/some/kind/of/a/path/here",
		Resources: &configs.Resources{},
	}
	m, err := NewManager(cg, nil)
	if err != nil {
		b.Fatal(err)
	}
	err = m.Apply(-1)
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		_ = m.Destroy()
	}()

	var st *cgroups.Stats

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st, err = m.GetStats()
		if err != nil {
			b.Fatal(err)
		}
	}
	if st.CpuStats.CpuUsage.TotalUsage != 0 {
		b.Fatalf("stats: %+v", st)
	}
}

// +build linux

package fs

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestInvalidCgroupPath(t *testing.T) {
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("cgroup v2 is not supported")
	}

	root, err := getCgroupRoot()
	if err != nil {
		t.Fatalf("couldn't get cgroup root: %v", err)
	}

	testCases := []struct {
		test               string
		path, name, parent string
	}{
		{
			test: "invalid cgroup path",
			path: "../../../../../../../../../../some/path",
		},
		{
			test: "invalid absolute cgroup path",
			path: "/../../../../../../../../../../some/path",
		},
		{
			test:   "invalid cgroup parent",
			parent: "../../../../../../../../../../some/path",
			name:   "name",
		},
		{
			test:   "invalid absolute cgroup parent",
			parent: "/../../../../../../../../../../some/path",
			name:   "name",
		},
		{
			test:   "invalid cgroup name",
			parent: "parent",
			name:   "../../../../../../../../../../some/path",
		},
		{
			test:   "invalid absolute cgroup name",
			parent: "parent",
			name:   "/../../../../../../../../../../some/path",
		},
		{
			test:   "invalid cgroup name and parent",
			parent: "../../../../../../../../../../some/path",
			name:   "../../../../../../../../../../some/path",
		},
		{
			test:   "invalid absolute cgroup name and parent",
			parent: "/../../../../../../../../../../some/path",
			name:   "/../../../../../../../../../../some/path",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
			config := &configs.Cgroup{Path: tc.path, Name: tc.name, Parent: tc.parent}

			data, err := getCgroupData(config, 0)
			if err != nil {
				t.Fatalf("couldn't get cgroup data: %v", err)
			}

			// Make sure the final innerPath doesn't go outside the cgroup mountpoint.
			if strings.HasPrefix(data.innerPath, "..") {
				t.Errorf("SECURITY: cgroup innerPath is outside cgroup mountpoint!")
			}

			// Double-check, using an actual cgroup.
			deviceRoot := filepath.Join(root, "devices")
			devicePath, err := data.path("devices")
			if err != nil {
				t.Fatalf("couldn't get cgroup path: %v", err)
			}
			if !strings.HasPrefix(devicePath, deviceRoot) {
				t.Errorf("SECURITY: cgroup path() is outside cgroup mountpoint!")
			}
		})
	}
}

func TestTryDefaultCgroupRoot(t *testing.T) {
	res := tryDefaultCgroupRoot()
	exp := defaultCgroupRoot
	if cgroups.IsCgroup2UnifiedMode() {
		// checking that tryDefaultCgroupRoot does return ""
		// in case /sys/fs/cgroup is not cgroup v1 root dir.
		exp = ""
	}
	if res != exp {
		t.Errorf("tryDefaultCgroupRoot: want %q, got %q", exp, res)
	}
}

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
	m := NewManager(cg, nil, false)
	err := m.Apply(-1)
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

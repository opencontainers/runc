package manager

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// TestNilResources checks that a cgroup manager do not panic when
// config.Resources is nil. While it does not make sense to use a
// manager with no resources, it should not result in a panic.
//
// This tests either v1 or v2 fs cgroup manager, depending on which
// cgroup version is available.
func TestNilResources(t *testing.T) {
	testNilResources(t, false)
}

// TestNilResourcesSystemd is the same as TestNilResources,
// only checking the systemd cgroup manager.
func TestNilResourcesSystemd(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("requires systemd")
	}
	testNilResources(t, true)
}

func testNilResources(t *testing.T, systemd bool) {
	cg := &configs.Cgroup{} // .Resources is nil
	cg.Systemd = systemd
	mgr, err := New(cg)
	if err != nil {
		// Some managers require non-nil Resources during
		// instantiation -- provide and retry. In such case
		// we're mostly testing Set(nil) below.
		cg.Resources = &configs.Resources{}
		mgr, err = New(cg)
		if err != nil {
			t.Fatal(err)
		}
	}
	_ = mgr.Apply(-1)
	_ = mgr.Set(nil)
	_ = mgr.Freeze(configs.Thawed)
	_ = mgr.Exists()
	_, _ = mgr.GetAllPids()
	_, _ = mgr.GetCgroups()
	_, _ = mgr.GetFreezerState()
	_ = mgr.Path("")
	_ = mgr.GetPaths()
	_, _ = mgr.GetStats()
	_, _ = mgr.OOMKillCount()
	_ = mgr.Destroy()
}

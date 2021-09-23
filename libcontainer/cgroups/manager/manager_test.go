package manager

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// TestNilResources checks that a cgroup manager do not panic when
// config.Resources is nil. While it does not make sense to use a
// manager with no resources, it should not result in a panic.
//
// This tests either v1 or v2 managers (both fs and systemd),
// depending on what cgroup version is available on the host.
func TestNilResources(t *testing.T) {
	for _, sd := range []bool{false, true} {
		cg := &configs.Cgroup{} // .Resources is nil
		cg.Systemd = sd
		mgr, err := New(cg)
		if err != nil {
			// Some managers require non-nil Resources during
			// instantiation -- provide and retry. In such case
			// we're mostly testing Set(nil) below.
			cg.Resources = &configs.Resources{}
			mgr, err = New(cg)
			if err != nil {
				t.Error(err)
				continue
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
}

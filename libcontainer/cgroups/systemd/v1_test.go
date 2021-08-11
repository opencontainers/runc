package systemd

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestFreezeBeforeSet(t *testing.T) {
	requireV1(t)

	testCases := []struct {
		desc string
		// Test input.
		cg        *configs.Cgroup
		preFreeze bool
		// Expected output.
		freeze, thaw bool
	}{
		{
			// A slice with SkipDevices.
			desc: "slice+skip-devices",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_1.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			freeze: false,
			thaw:   false,
		},
		{
			// A scope with SkipDevices. Not a realistic scenario with runc
			// (as container can't have SkipDevices == true), but possible
			// for a standalone cgroup manager.
			desc: "scope+skip-devices",
			cg: &configs.Cgroup{
				ScopePrefix: "test",
				Name:        "testFreeze2",
				Parent:      "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			freeze: false,
			thaw:   false,
		},
		{
			// A slice that is about to be frozen in Set.
			desc: "slice-to-be-frozen",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_3.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					Freezer: configs.Frozen,
				},
			},
			freeze: true,
			thaw:   false,
		},
		{
			// A pre-frozen slice that should stay frozen.
			desc: "slice+pre-frozen",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_4.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					Freezer: configs.Frozen,
				},
			},
			preFreeze: true,
			freeze:    false,
			thaw:      false,
		},
		{
			// A scope that is pre-frozen.
			desc: "scope+pre-frozen",
			cg: &configs.Cgroup{
				ScopePrefix: "test",
				Name:        "testFreeze5",
				Parent:      "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			preFreeze: true,
			freeze:    false,
			thaw:      false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			m := NewLegacyManager(tc.cg, nil)
			defer m.Destroy() //nolint:errcheck
			// Create systemd unit.
			pid := -1
			if strings.HasSuffix(getUnitName(tc.cg), ".scope") {
				// Scopes require a process inside.
				cmd := exec.Command("bash", "-c", "sleep 1h")
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				pid = cmd.Process.Pid
				// Make sure to not leave a zombie.
				defer func() {
					// These may fail, we don't care.
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}()
			}
			if err := m.Apply(pid); err != nil {
				t.Fatal(err)
			}
			if tc.preFreeze {
				if err := m.Freeze(configs.Frozen); err != nil {
					t.Error(err)
					return // no more checks
				}
			}
			lm := m.(*legacyManager)
			freeze, thaw, err := lm.freezeBeforeSet(getUnitName(tc.cg), tc.cg.Resources)
			if err != nil {
				t.Error(err)
				return // no more checks
			}
			if freeze != tc.freeze || thaw != tc.thaw {
				t.Errorf("expected freeze: %v, thaw: %v, got freeze: %v, thaw: %v",
					tc.freeze, tc.thaw, freeze, thaw)
			}
			// Destroy() timeouts on a frozen container, so we need to thaw it.
			if tc.preFreeze {
				if err := m.Freeze(configs.Thawed); err != nil {
					t.Error(err)
				}
			}
			// Destroy() do not kill processes in cgroup, so we should.
			if pid != -1 {
				if err = unix.Kill(pid, unix.SIGKILL); err != nil {
					t.Errorf("unable to kill pid %d: %s", pid, err)
				}
			}
			// Not really needed, but may help catch some bugs.
			if err := m.Destroy(); err != nil {
				t.Errorf("destroy: %s", err)
			}
		})
	}
}

// requireV1 skips the test unless a set of requirements (cgroup v1,
// systemd, root) is met.
func requireV1(t *testing.T) {
	t.Helper()
	if cgroups.IsCgroup2UnifiedMode() {
		t.Skip("Test requires cgroup v1.")
	}
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}
}

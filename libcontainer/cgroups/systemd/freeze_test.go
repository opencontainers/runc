package systemd

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

func TestFreezeBeforeSet(t *testing.T) {
	requireV1(t)

	testCases := []struct {
		desc string
		// Test input.
		cg        *configs.Cgroup
		preFreeze bool
		// Expected values.
		// Before unit creation (Apply).
		freeze0, thaw0 bool
		// After unit creation.
		freeze1, thaw1 bool
	}{
		{
			// A slice with SkipDevices.
			desc: "slice,skip-devices",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_1.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			// Expected.
			freeze0: false,
			thaw0:   false,
			freeze1: false,
			thaw1:   false,
		},
		{
			// A scope with SkipDevices. Not a realistic scenario with runc
			// (as container can't have SkipDevices == true), but possible
			// for a standalone cgroup manager.
			desc: "scope,skip-devices",
			cg: &configs.Cgroup{
				ScopePrefix: "test",
				Name:        "testFreeze2",
				Parent:      "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			// Expected.
			freeze0: false,
			thaw0:   false,
			freeze1: false,
			thaw1:   false,
		},
		{
			// A slice that is about to be frozen in Set.
			desc: "slice,will-freeze",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_3.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					Freezer: configs.Frozen,
				},
			},
			// Expected.
			freeze0: true,
			thaw0:   false,
			freeze1: true,
			thaw1:   false,
		},
		{
			// A pre-frozen slice that should stay frozen.
			desc: "slice,pre-frozen,will-freeze",
			cg: &configs.Cgroup{
				Name:   "system-runc_test_freeze_4.slice",
				Parent: "system.slice",
				Resources: &configs.Resources{
					Freezer: configs.Frozen,
				},
			},
			preFreeze: true,
			// Expected.
			freeze0: true, // not actually frozen yet.
			thaw0:   false,
			freeze1: false,
			thaw1:   false,
		},
		{
			// A pre-frozen scope with skip devices set.
			desc: "scope,pre-frozen,skip-devices",
			cg: &configs.Cgroup{
				ScopePrefix: "test",
				Name:        "testFreeze5",
				Parent:      "system.slice",
				Resources: &configs.Resources{
					SkipDevices: true,
				},
			},
			preFreeze: true,
			// Expected.
			freeze0: false,
			thaw0:   false,
			freeze1: false,
			thaw1:   false,
		},
		{
			// A pre-frozen scope which will be thawed.
			desc: "scope,pre-frozen",
			cg: &configs.Cgroup{
				ScopePrefix: "test",
				Name:        "testFreeze6",
				Parent:      "system.slice",
				Resources:   &configs.Resources{},
			},
			preFreeze: true,
			// Expected.
			freeze0: true, // not actually frozen yet.
			thaw0:   true,
			freeze1: false,
			thaw1:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			m, err := NewLegacyManager(tc.cg, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer m.Destroy() //nolint:errcheck

			// Checks for a non-existent unit.
			freeze, thaw, err := m.freezeBeforeSet(getUnitName(tc.cg), tc.cg.Resources)
			if err != nil {
				t.Fatal(err)
			}
			if freeze != tc.freeze0 || thaw != tc.thaw0 {
				t.Errorf("before Apply (non-existent unit): expected freeze: %v, thaw: %v, got freeze: %v, thaw: %v",
					tc.freeze0, tc.thaw0, freeze, thaw)
			}

			// Create systemd unit.
			pid := -1
			if strings.HasSuffix(getUnitName(tc.cg), ".scope") {
				// Scopes require a process inside.
				cmd := exec.Command("bash", "-c", "sleep 1m")
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
			freeze, thaw, err = m.freezeBeforeSet(getUnitName(tc.cg), tc.cg.Resources)
			if err != nil {
				t.Error(err)
				return // no more checks
			}
			if freeze != tc.freeze1 || thaw != tc.thaw1 {
				t.Errorf("expected freeze: %v, thaw: %v, got freeze: %v, thaw: %v",
					tc.freeze1, tc.thaw1, freeze, thaw)
			}
			// Destroy() timeouts on a frozen container, so we need to thaw it.
			if tc.preFreeze {
				if err := m.Freeze(configs.Thawed); err != nil {
					t.Error(err)
				}
			}
			// Destroy() does not kill processes in cgroup, so we should.
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

func TestFreezePodCgroup(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	podConfig := &configs.Cgroup{
		Parent: "system.slice",
		Name:   "system-runc_test_pod.slice",
		Resources: &configs.Resources{
			SkipDevices: true,
			Freezer:     configs.Frozen,
		},
	}
	// Create a "pod" cgroup (a systemd slice to hold containers),
	// which is frozen initially.
	pm := newManager(t, podConfig)
	if err := pm.Apply(-1); err != nil {
		t.Fatal(err)
	}

	if err := pm.Set(podConfig.Resources); err != nil {
		t.Fatal(err)
	}

	// Check the pod is frozen.
	pf, err := pm.GetFreezerState()
	if err != nil {
		t.Fatal(err)
	}
	if pf != configs.Frozen {
		t.Fatalf("expected pod to be frozen, got %v", pf)
	}

	// Create a "container" within the "pod" cgroup.
	// This is not a real container, just a process in the cgroup.
	containerConfig := &configs.Cgroup{
		Parent:      "system-runc_test_pod.slice",
		ScopePrefix: "test",
		Name:        "inner-container",
		Resources:   &configs.Resources{},
	}

	cmd := exec.Command("bash", "-c", "while read; do echo $REPLY; done")
	cmd.Env = append(os.Environ(), "LANG=C")

	// Setup stdin.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = stdinR

	// Setup stdout.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout = stdoutW
	rdr := bufio.NewReader(stdoutR)

	// Setup stderr.
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Start()
	stdinR.Close()
	stdoutW.Close()
	defer func() {
		_ = stdinW.Close()
		_ = stdoutR.Close()
	}()
	if err != nil {
		t.Fatal(err)
	}
	// Make sure to not leave a zombie.
	defer func() {
		// These may fail, we don't care.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Put the process into a cgroup.
	cm := newManager(t, containerConfig)

	if err := cm.Apply(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if err := cm.Set(containerConfig.Resources); err != nil {
		t.Fatal(err)
	}
	// Check that we put the "container" into the "pod" cgroup.
	if !strings.HasPrefix(cm.Path("freezer"), pm.Path("freezer")) {
		t.Fatalf("expected container cgroup path %q to be under pod cgroup path %q",
			cm.Path("freezer"), pm.Path("freezer"))
	}
	// Check the container is not reported as frozen despite the frozen parent.
	cf, err := cm.GetFreezerState()
	if err != nil {
		t.Fatal(err)
	}
	if cf != configs.Thawed {
		t.Fatalf("expected container to be thawed, got %v", cf)
	}

	// Unfreeze the pod.
	if err := pm.Freeze(configs.Thawed); err != nil {
		t.Fatal(err)
	}

	cf, err = cm.GetFreezerState()
	if err != nil {
		t.Fatal(err)
	}
	if cf != configs.Thawed {
		t.Fatalf("expected container to be thawed, got %v", cf)
	}

	// Check the "container" works.
	marker := "one two\n"
	_, err = stdinW.WriteString(marker)
	if err != nil {
		t.Fatal(err)
	}
	reply, err := rdr.ReadString('\n')
	if err != nil {
		t.Fatalf("reading from container: %v", err)
	}
	if reply != marker {
		t.Fatalf("expected %q, got %q", marker, reply)
	}
}

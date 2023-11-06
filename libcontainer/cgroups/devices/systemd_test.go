package devices

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/opencontainers/runc/internal/testutil"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
)

// TestPodSkipDevicesUpdate checks that updating a pod having SkipDevices: true
// does not result in spurious "permission denied" errors in a container
// running under the pod. The test is somewhat similar in nature to the
// @test "update devices [minimal transition rules]" in tests/integration,
// but uses a pod.
func TestPodSkipDevicesUpdate(t *testing.T) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}
	// https://github.com/opencontainers/runc/issues/3743.
	testutil.SkipOnCentOS(t, "Flaky (#3743)", 7)

	podName := "system-runc_test_pod" + t.Name() + ".slice"
	podConfig := &configs.Cgroup{
		Systemd: true,
		Parent:  "system.slice",
		Name:    podName,
		Resources: &configs.Resources{
			PidsLimit:   42,
			Memory:      32 * 1024 * 1024,
			SkipDevices: true,
		},
	}
	// Create "pod" cgroup (a systemd slice to hold containers).
	pm := newManager(t, podConfig)
	if err := pm.Apply(-1); err != nil {
		t.Fatal(err)
	}
	if err := pm.Set(podConfig.Resources); err != nil {
		t.Fatal(err)
	}

	containerConfig := &configs.Cgroup{
		Parent:      podName,
		ScopePrefix: "test",
		Name:        "PodSkipDevicesUpdate",
		Resources: &configs.Resources{
			Devices: []*devices.Rule{
				// Allow access to /dev/null.
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       3,
					Permissions: "rwm",
					Allow:       true,
				},
			},
		},
	}

	// Create a "container" within the "pod" cgroup.
	// This is not a real container, just a process in the cgroup.
	cmd := exec.Command("sleep", "infinity")
	cmd.Env = append(os.Environ(), "LANG=C")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
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
	// Check that we put the "container" into the "pod" cgroup.
	if !strings.HasPrefix(cm.Path("devices"), pm.Path("devices")) {
		t.Fatalf("expected container cgroup path %q to be under pod cgroup path %q",
			cm.Path("devices"), pm.Path("devices"))
	}
	if err := cm.Set(containerConfig.Resources); err != nil {
		t.Fatal(err)
	}

	// Now update the pod a few times.
	for i := 0; i < 42; i++ {
		podConfig.Resources.PidsLimit++
		podConfig.Resources.Memory += 1024 * 1024
		if err := pm.Set(podConfig.Resources); err != nil {
			t.Fatal(err)
		}
	}
	// Kill the "container".
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	_ = cmd.Wait()

	// "Container" stderr should be empty.
	if stderr.Len() != 0 {
		t.Fatalf("container stderr not empty: %s", stderr.String())
	}
}

func testSkipDevices(t *testing.T, skipDevices bool, expected []string) {
	if !systemd.IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}
	// https://github.com/opencontainers/runc/issues/3743.
	testutil.SkipOnCentOS(t, "Flaky (#3743)", 7)

	podConfig := &configs.Cgroup{
		Parent: "system.slice",
		Name:   "system-runc_test_pods.slice",
		Resources: &configs.Resources{
			SkipDevices: skipDevices,
		},
	}
	// Create "pods" cgroup (a systemd slice to hold containers).
	pm := newManager(t, podConfig)
	if err := pm.Apply(-1); err != nil {
		t.Fatal(err)
	}
	if err := pm.Set(podConfig.Resources); err != nil {
		t.Fatal(err)
	}

	config := &configs.Cgroup{
		Parent:      "system-runc_test_pods.slice",
		ScopePrefix: "test",
		Name:        "SkipDevices",
		Resources: &configs.Resources{
			Devices: []*devices.Rule{
				// Allow access to /dev/full only.
				{
					Type:        devices.CharDevice,
					Major:       1,
					Minor:       7,
					Permissions: "rwm",
					Allow:       true,
				},
			},
		},
	}

	// Create a "container" within the "pods" cgroup.
	// This is not a real container, just a process in the cgroup.
	cmd := exec.Command("bash", "-c", "read; echo > /dev/full; cat /dev/null; true")
	cmd.Env = append(os.Environ(), "LANG=C")
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = stdinR
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err = cmd.Start()
	stdinR.Close()
	defer stdinW.Close()
	if err != nil {
		t.Fatal(err)
	}
	// Make sure to not leave a zombie.
	defer func() {
		// These may fail, we don't care.
		_, _ = stdinW.WriteString("hey\n")
		_ = cmd.Wait()
	}()

	// Put the process into a cgroup.
	m := newManager(t, config)
	if err := m.Apply(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	// Check that we put the "container" into the "pod" cgroup.
	if !strings.HasPrefix(m.Path("devices"), pm.Path("devices")) {
		t.Fatalf("expected container cgroup path %q to be under pod cgroup path %q",
			m.Path("devices"), pm.Path("devices"))
	}
	if err := m.Set(config.Resources); err != nil {
		// failed to write "c 1:7 rwm": write /sys/fs/cgroup/devices/system.slice/system-runc_test_pods.slice/test-SkipDevices.scope/devices.allow: operation not permitted
		if skipDevices == false && strings.HasSuffix(err.Error(), "/devices.allow: operation not permitted") {
			// Cgroup v1 devices controller gives EPERM on trying
			// to enable devices that are not enabled
			// (skipDevices=false) in a parent cgroup.
			// If this happens, test is passing.
			return
		}
		t.Fatal(err)
	}

	// Check that we can access /dev/full but not /dev/zero.
	if _, err := stdinW.WriteString("wow\n"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	for _, exp := range expected {
		if !strings.Contains(stderr.String(), exp) {
			t.Errorf("expected %q, got: %s", exp, stderr.String())
		}
	}
}

func TestSkipDevicesTrue(t *testing.T) {
	testSkipDevices(t, true, []string{
		"echo: write error: No space left on device",
		"cat: /dev/null: Operation not permitted",
	})
}

func TestSkipDevicesFalse(t *testing.T) {
	// If SkipDevices is not set for the parent slice, access to both
	// devices should fail. This is done to assess the test correctness.
	// For cgroup v1, we check for m.Set returning EPERM.
	// For cgroup v2, we check for the errors below.
	testSkipDevices(t, false, []string{
		"/dev/full: Operation not permitted",
		"cat: /dev/null: Operation not permitted",
	})
}

func testFindDeviceGroup() error {
	const (
		major = 136
		group = "char-pts"
	)
	res, err := findDeviceGroup(devices.CharDevice, major)
	if res != group || err != nil {
		return fmt.Errorf("expected %v, nil, got %v, %w", group, res, err)
	}
	return nil
}

func TestFindDeviceGroup(t *testing.T) {
	if err := testFindDeviceGroup(); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkFindDeviceGroup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if err := testFindDeviceGroup(); err != nil {
			b.Fatal(err)
		}
	}
}

func newManager(t *testing.T, config *configs.Cgroup) (m cgroups.Manager) {
	t.Helper()
	var err error

	if cgroups.IsCgroup2UnifiedMode() {
		m, err = systemd.NewUnifiedManager(config, "")
	} else {
		m, err = systemd.NewLegacyManager(config, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Destroy() })

	return m
}

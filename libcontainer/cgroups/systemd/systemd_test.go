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
	"github.com/opencontainers/runc/libcontainer/devices"
)

func TestSystemdVersion(t *testing.T) {
	systemdVersionTests := []struct {
		verStr      string
		expectedVer int
		expectErr   bool
	}{
		{`"219"`, 219, false},
		{`"v245.4-1.fc32"`, 245, false},
		{`"241-1"`, 241, false},
		{`"v241-1"`, 241, false},
		{"NaN", 0, true},
		{"", 0, true},
	}
	for _, sdTest := range systemdVersionTests {
		ver, err := systemdVersionAtoi(sdTest.verStr)
		if !sdTest.expectErr && err != nil {
			t.Errorf("systemdVersionAtoi(%s); want nil; got %v", sdTest.verStr, err)
		}
		if sdTest.expectErr && err == nil {
			t.Errorf("systemdVersionAtoi(%s); wanted failure; got nil", sdTest.verStr)
		}
		if ver != sdTest.expectedVer {
			t.Errorf("systemdVersionAtoi(%s); want %d; got %d", sdTest.verStr, sdTest.expectedVer, ver)
		}
	}
}

func TestValidUnitTypes(t *testing.T) {
	testCases := []struct {
		unitName         string
		expectedUnitType string
	}{
		{"system.slice", "Slice"},
		{"kubepods.slice", "Slice"},
		{"testing-container:ab.scope", "Scope"},
	}
	for _, sdTest := range testCases {
		unitType := getUnitType(sdTest.unitName)
		if unitType != sdTest.expectedUnitType {
			t.Errorf("getUnitType(%s); want %q; got %q", sdTest.unitName, sdTest.expectedUnitType, unitType)
		}
	}
}

func newManager(config *configs.Cgroup) cgroups.Manager {
	if cgroups.IsCgroup2UnifiedMode() {
		return NewUnifiedManager(config, "", false)
	}
	return NewLegacyManager(config, nil)
}

// TestPodSkipDevicesUpdate checks that updating a pod having SkipDevices: true
// does not result in spurious "permission denied" errors in a container
// running under the pod. The test is somewhat similar in nature to the
// @test "update devices [minimal transition rules]" in tests/integration,
// but uses a pod.
func TestPodSkipDevicesUpdate(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	podName := "system-runc_test_pod" + t.Name() + ".slice"
	podConfig := &configs.Cgroup{
		Parent: "system.slice",
		Name:   podName,
		Resources: &configs.Resources{
			PidsLimit:   42,
			Memory:      32 * 1024 * 1024,
			SkipDevices: true,
		},
	}
	// Create "pod" cgroup (a systemd slice to hold containers).
	pm := newManager(podConfig)
	defer pm.Destroy() //nolint:errcheck
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
	cmd := exec.Command("bash", "-c", "while true; do echo > /dev/null; done")
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
	cm := newManager(containerConfig)
	defer cm.Destroy() //nolint:errcheck

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
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	podConfig := &configs.Cgroup{
		Parent: "system.slice",
		Name:   "system-runc_test_pods.slice",
		Resources: &configs.Resources{
			SkipDevices: skipDevices,
		},
	}
	// Create "pods" cgroup (a systemd slice to hold containers).
	pm := newManager(podConfig)
	defer pm.Destroy() //nolint:errcheck
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
	m := newManager(config)
	defer m.Destroy() //nolint:errcheck

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

func TestUnitExistsIgnored(t *testing.T) {
	if !IsRunningSystemd() {
		t.Skip("Test requires systemd.")
	}
	if os.Geteuid() != 0 {
		t.Skip("Test requires root.")
	}

	podConfig := &configs.Cgroup{
		Parent:    "system.slice",
		Name:      "system-runc_test_exists.slice",
		Resources: &configs.Resources{},
	}
	// Create "pods" cgroup (a systemd slice to hold containers).
	pm := newManager(podConfig)
	defer pm.Destroy() //nolint:errcheck

	// create twice to make sure "UnitExists" error is ignored.
	for i := 0; i < 2; i++ {
		if err := pm.Apply(-1); err != nil {
			t.Fatal(err)
		}
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
	pm := newManager(podConfig)
	defer pm.Destroy() //nolint:errcheck
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
		Name:        "inner-contianer",
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
	cm := newManager(containerConfig)
	defer cm.Destroy() //nolint:errcheck

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

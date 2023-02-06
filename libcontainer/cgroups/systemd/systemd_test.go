package systemd

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func newManager(t *testing.T, config *configs.Cgroup) (m cgroups.Manager) {
	t.Helper()
	var err error

	if cgroups.IsCgroup2UnifiedMode() {
		m, err = NewUnifiedManager(config, "")
	} else {
		m, err = NewLegacyManager(config, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Destroy() })

	return m
}

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
		{`333.45"`, 333, false},
		{`v321-0`, 321, false},
		{"NaN", -1, true},
		{"", -1, true},
		{"v", -1, true},
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
	pm := newManager(t, podConfig)

	// create twice to make sure "UnitExists" error is ignored.
	for i := 0; i < 2; i++ {
		if err := pm.Apply(-1); err != nil {
			t.Fatal(err)
		}
	}
}

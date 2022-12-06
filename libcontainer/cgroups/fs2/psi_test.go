package fs2

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const examplePSIData = `some avg10=1.71 avg60=2.36 avg300=2.57 total=230548833
full avg10=1.00 avg60=1.01 avg300=1.00 total=157622356`

func createFloatPtr(f float64) *float64 {
	return &f
}
func createUInt64Ptr(i uint64) *uint64 {
	return &i
}

func TestStatCPUPSI(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	statPath := filepath.Join(fakeCgroupDir, "cpu.pressure")

	if err := os.WriteFile(statPath, []byte(examplePSIData), 0o644); err != nil {
		t.Fatal(err)
	}

	var psi cgroups.PSIStats
	if err := statPSI(fakeCgroupDir, "cpu.pressure", &psi); err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(psi, cgroups.PSIStats{
		Some: &cgroups.PSIData{
			Avg10:  createFloatPtr(1.71),
			Avg60:  createFloatPtr(2.36),
			Avg300: createFloatPtr(2.57),
			Total:  createUInt64Ptr(230548833),
		},
		Full: &cgroups.PSIData{
			Avg10:  createFloatPtr(1.00),
			Avg60:  createFloatPtr(1.01),
			Avg300: createFloatPtr(1.00),
			Total:  createUInt64Ptr(157622356),
		},
	}) {
		t.Errorf("unexpected PSI result: %+v", psi)
	}
}

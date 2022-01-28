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

func TestStatCPUPsi(t *testing.T) {
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
		Some: cgroups.PSIData{
			Avg10:  1.71,
			Avg60:  2.36,
			Avg300: 2.57,
			Total:  230548833,
		},
		Full: cgroups.PSIData{
			Avg10:  1.00,
			Avg60:  1.01,
			Avg300: 1.00,
			Total:  157622356,
		},
	}) {
		t.Errorf("unexpected psi result: %+v", psi)
	}
}

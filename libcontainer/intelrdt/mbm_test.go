package intelrdt

import (
	"path/filepath"
	"testing"
)

func TestGetMBMNumaNodeStats(t *testing.T) {
	mocksNUMANodesToCreate := []string{"mon_l3_00", "mon_l3_01"}

	mocksFilesToCreate := map[string]uint64{
		"mbm_total_bytes": 9123911,
		"mbm_local_bytes": 2361361,
	}

	mockedL3_MON := mockResctrlL3_MON(t, mocksNUMANodesToCreate, mocksFilesToCreate)

	t.Run("Gather mbm", func(t *testing.T) {
		enabledMonFeatures.mbmTotalBytes = true
		enabledMonFeatures.mbmLocalBytes = true

		stats := make([]MBMNumaNodeStats, 0, len(mocksNUMANodesToCreate))
		for _, numa := range mocksNUMANodesToCreate {
			other, err := getMBMNumaNodeStats(filepath.Join(mockedL3_MON, "mon_data", numa))
			if err != nil {
				t.Fatal(err)
			}
			stats = append(stats, *other)
		}

		expectedStats := MBMNumaNodeStats{
			MBMTotalBytes: mocksFilesToCreate["mbm_total_bytes"],
			MBMLocalBytes: mocksFilesToCreate["mbm_local_bytes"],
		}

		checkMBMStatCorrection(stats[0], expectedStats, t)
		checkMBMStatCorrection(stats[1], expectedStats, t)
	})
}

func checkMBMStatCorrection(got MBMNumaNodeStats, expected MBMNumaNodeStats, t *testing.T) {
	if got.MBMTotalBytes != expected.MBMTotalBytes {
		t.Fatalf("Wrong value of mbm_total_bytes. Expected: %v but got: %v",
			expected.MBMTotalBytes,
			got.MBMTotalBytes)
	}

	if got.MBMLocalBytes != expected.MBMLocalBytes {
		t.Fatalf("Wrong value of mbm_local_bytes. Expected: %v but got: %v",
			expected.MBMLocalBytes,
			got.MBMLocalBytes)
	}
}

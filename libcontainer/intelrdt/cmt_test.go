package intelrdt

import (
	"path/filepath"
	"testing"
)

func TestGetCMTNumaNodeStats(t *testing.T) {
	mocksNUMANodesToCreate := []string{"mon_l3_00", "mon_l3_01"}

	mocksFilesToCreate := map[string]uint64{
		"llc_occupancy": 9123911,
	}

	mockedL3_MON := mockResctrlL3_MON(t, mocksNUMANodesToCreate, mocksFilesToCreate)

	t.Run("Gather mbm", func(t *testing.T) {
		enabledMonFeatures.llcOccupancy = true

		stats := make([]CMTNumaNodeStats, 0, len(mocksNUMANodesToCreate))
		for _, numa := range mocksNUMANodesToCreate {
			other, err := getCMTNumaNodeStats(filepath.Join(mockedL3_MON, "mon_data", numa))
			if err != nil {
				t.Fatal(err)
			}
			stats = append(stats, *other)
		}

		expectedStats := CMTNumaNodeStats{
			LLCOccupancy: mocksFilesToCreate["llc_occupancy"],
		}

		checkCMTStatCorrection(stats[0], expectedStats, t)
		checkCMTStatCorrection(stats[1], expectedStats, t)
	})
}

func checkCMTStatCorrection(got CMTNumaNodeStats, expected CMTNumaNodeStats, t *testing.T) {
	if got.LLCOccupancy != expected.LLCOccupancy {
		t.Fatalf("Wrong value of `llc_occupancy`. Expected: %v but got: %v",
			expected.LLCOccupancy,
			got.LLCOccupancy)
	}
}

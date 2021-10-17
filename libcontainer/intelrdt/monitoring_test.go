package intelrdt

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseMonFeatures(t *testing.T) {
	t.Run("All features available", func(t *testing.T) {
		parsedMonFeatures, err := parseMonFeatures(
			strings.NewReader("mbm_total_bytes\nmbm_local_bytes\nllc_occupancy"))
		if err != nil {
			t.Errorf("Error while parsing mon features err = %v", err)
		}

		expectedMonFeatures := monFeatures{true, true, true}

		if parsedMonFeatures != expectedMonFeatures {
			t.Error("Cannot gather all features!")
		}
	})

	t.Run("No features available", func(t *testing.T) {
		parsedMonFeatures, err := parseMonFeatures(strings.NewReader(""))
		if err != nil {
			t.Errorf("Error while parsing mon features err = %v", err)
		}

		expectedMonFeatures := monFeatures{false, false, false}

		if parsedMonFeatures != expectedMonFeatures {
			t.Error("Expected no features available but there is any!")
		}
	})
}

func mockResctrlL3_MON(t *testing.T, NUMANodes []string, mocks map[string]uint64) string {
	t.Helper()
	testDir := t.TempDir()
	monDataPath := filepath.Join(testDir, "mon_data")

	for _, numa := range NUMANodes {
		numaPath := filepath.Join(monDataPath, numa)
		err := os.MkdirAll(numaPath, 0o700)
		if err != nil {
			t.Fatal(err)
		}

		for fileName, value := range mocks {
			err := os.WriteFile(filepath.Join(numaPath, fileName), []byte(strconv.FormatUint(value, 10)), 0o644)
			if err != nil {
				t.Fatal(err)
			}
		}

	}

	return testDir
}

func TestGetMonitoringStats(t *testing.T) {
	enabledMonFeatures.mbmTotalBytes = true
	enabledMonFeatures.mbmLocalBytes = true
	enabledMonFeatures.llcOccupancy = true
	mbmEnabled = true
	cmtEnabled = true

	mocksNUMANodesToCreate := []string{"mon_l3_00", "mon_l3_01"}

	mocksFilesToCreate := map[string]uint64{
		"mbm_total_bytes": 9123911,
		"mbm_local_bytes": 2361361,
		"llc_occupancy":   123331,
	}

	mockedL3_MON := mockResctrlL3_MON(t, mocksNUMANodesToCreate, mocksFilesToCreate)

	t.Run("Gather monitoring stats", func(t *testing.T) {
		var stats Stats
		err := getMonitoringStats(mockedL3_MON, &stats)
		if err != nil {
			t.Fatal(err)
		}

		expectedMBMStats := MBMNumaNodeStats{
			MBMTotalBytes: mocksFilesToCreate["mbm_total_bytes"],
			MBMLocalBytes: mocksFilesToCreate["mbm_local_bytes"],
		}

		expectedCMTStats := CMTNumaNodeStats{LLCOccupancy: mocksFilesToCreate["llc_occupancy"]}

		for _, gotMBMStat := range *stats.MBMStats {
			checkMBMStatCorrection(gotMBMStat, expectedMBMStats, t)
		}

		for _, gotCMTStat := range *stats.CMTStats {
			checkCMTStatCorrection(gotCMTStat, expectedCMTStats, t)
		}
	})
}

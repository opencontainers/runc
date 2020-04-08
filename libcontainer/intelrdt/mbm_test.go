// +build linux

package intelrdt

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestParseMonFeatures(t *testing.T) {
	t.Run("All features available", func(t *testing.T) {
		parsedMonFeatures, err := parseMonFeatures(
			strings.NewReader("mbm_total_bytes\nmbm_local_bytes"))
		if err != nil {
			t.Errorf("Error while parsing mon features err = %v", err)
		}

		expectedMonFeatures := monFeatures{true, true}

		if parsedMonFeatures != expectedMonFeatures {
			t.Error("Cannot gather all features!")
		}
	})

	t.Run("No features available", func(t *testing.T) {
		parsedMonFeatures, err := parseMonFeatures(strings.NewReader(""))

		if err != nil {
			t.Errorf("Error while parsing mon features err = %v", err)
		}

		expectedMonFeatures := monFeatures{false, false}

		if parsedMonFeatures != expectedMonFeatures {
			t.Error("Expected no features available but there is any!")
		}
	})
}

func mockMBM(NUMANodes []string, mocks map[string]uint64) (string, error) {
	testDir, err := ioutil.TempDir("", "rdt_mbm_test")
	if err != nil {
		return "", err
	}
	monDataPath := filepath.Join(testDir, "mon_data")

	for _, numa := range NUMANodes {
		numaPath := filepath.Join(monDataPath, numa)
		err = os.MkdirAll(numaPath, os.ModePerm)
		if err != nil {
			return "", err
		}

		for fileName, value := range mocks {
			err := ioutil.WriteFile(filepath.Join(numaPath, fileName), []byte(strconv.FormatUint(value, 10)), 777)
			if err != nil {
				return "", err
			}
		}

	}

	return testDir, nil
}

func TestGetMbmStats(t *testing.T) {
	mocksNUMANodesToCreate := []string{"mon_l3_00", "mon_l3_01"}

	mocksFilesToCreate := map[string]uint64{
		"mbm_total_bytes": 9123911,
		"mbm_local_bytes": 2361361,
	}

	mockedMBM, err := mockMBM(mocksNUMANodesToCreate, mocksFilesToCreate)

	defer func() {
		err := os.RemoveAll(mockedMBM)
		if err != nil {
			t.Fatal(err)
		}
	}()

	if err != nil {
		t.Fatal(err)
	}

	t.Run("Gather mbm", func(t *testing.T) {
		enabledMonFeatures.mbmTotalBytes = true
		enabledMonFeatures.mbmLocalBytes = true

		stats, err := getMBMStats(mockedMBM)
		if err != nil {
			t.Fatal(err)
		}

		if len(*stats) != len(mocksNUMANodesToCreate) {
			t.Fatalf("Wrong number of stats slices from NUMA nodes. Expected: %v but got: %v",
				len(mocksNUMANodesToCreate), len(*stats))
		}

		checkStatCorrection := func(got MBMNumaNodeStats, expected MBMNumaNodeStats, t *testing.T) {
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

		expectedStats := MBMNumaNodeStats{
			MBMTotalBytes: mocksFilesToCreate["mbm_total_bytes"],
			MBMLocalBytes: mocksFilesToCreate["mbm_local_bytes"],
		}

		checkStatCorrection((*stats)[0], expectedStats, t)
		checkStatCorrection((*stats)[1], expectedStats, t)
	})
}

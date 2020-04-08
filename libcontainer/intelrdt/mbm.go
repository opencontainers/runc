// +build linux

package intelrdt

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var (
	// The flag to indicate if Intel RDT/MBM is enabled
	isMbmEnabled bool

	enabledMonFeatures monFeatures
)

type monFeatures struct {
	mbmTotalBytes bool
	mbmLocalBytes bool
}

// Check if Intel RDT/MBM is enabled
func IsMbmEnabled() bool {
	return isMbmEnabled
}

func getMonFeatures(intelRdtRoot string) (monFeatures, error) {
	file, err := os.Open(filepath.Join(intelRdtRoot, "info", "L3_MON", "mon_features"))
	defer file.Close()
	if err != nil {
		return monFeatures{}, err
	}
	return parseMonFeatures(file)
}

func parseMonFeatures(reader io.Reader) (monFeatures, error) {
	scanner := bufio.NewScanner(reader)

	monFeatures := monFeatures{}

	for scanner.Scan() {

		switch feature := scanner.Text(); feature {

		case "mbm_total_bytes":
			monFeatures.mbmTotalBytes = true
		case "mbm_local_bytes":
			monFeatures.mbmLocalBytes = true
		default:
			logrus.Warnf("Unsupported Intel RDT monitoring feature: %s", feature)
		}
	}

	return monFeatures, scanner.Err()
}

func getMBMStats(containerPath string) (*[]MBMNumaNodeStats, error) {
	var mbmStats []MBMNumaNodeStats

	numaFiles, err := ioutil.ReadDir(filepath.Join(containerPath, "mon_data"))
	if err != nil {
		return &mbmStats, err
	}

	for _, file := range numaFiles {
		if file.IsDir() {
			numaStats, err := getMBMNumaNodeStats(filepath.Join(containerPath, "mon_data", file.Name()))
			if err != nil {
				return &mbmStats, nil
			}
			mbmStats = append(mbmStats, *numaStats)
		}
	}

	return &mbmStats, nil
}

func getMBMNumaNodeStats(numaPath string) (*MBMNumaNodeStats, error) {
	stats := &MBMNumaNodeStats{}
	if enabledMonFeatures.mbmTotalBytes {
		mbmTotalBytes, err := getIntelRdtParamUint(numaPath, "mbm_total_bytes")
		if err != nil {
			return nil, err
		}
		stats.MBMTotalBytes = mbmTotalBytes
	}

	if enabledMonFeatures.mbmLocalBytes {
		mbmLocalBytes, err := getIntelRdtParamUint(numaPath, "mbm_local_bytes")
		if err != nil {
			return nil, err
		}
		stats.MBMLocalBytes = mbmLocalBytes
	}

	return stats, nil
}

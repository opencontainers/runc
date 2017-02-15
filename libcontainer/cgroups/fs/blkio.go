// +build linux

package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type BlkioGroup struct {
	SupportV2 bool
}

func (s *BlkioGroup) Name() string {
	return "blkio"
}

func (s *BlkioGroup) Apply(d *cgroupData) error {
	_, err := d.join("blkio")
	if cgroups.IsV2Error(err) {
		s.SupportV2 = true
	} else if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *BlkioGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.BlkioWeight != 0 {
		name := "blkio.weight"
		defaultWeight := strconv.FormatUint(uint64(cgroup.Resources.BlkioWeight), 10)
		if s.SupportV2 {
			name = "io.weight"
			defaultWeight = "default " + defaultWeight
		}
		if err := writeFile(path, name, defaultWeight); err != nil {
			return err
		}
	}

	if cgroup.Resources.BlkioLeafWeight != 0 && !s.SupportV2 {
		if err := writeFile(path, "blkio.leaf_weight", strconv.FormatUint(uint64(cgroup.Resources.BlkioLeafWeight), 10)); err != nil {
			return err
		}
	}

	for _, wd := range cgroup.Resources.BlkioWeightDevice {
		name := "blkio.weight_device"
		if s.SupportV2 {
			name = "io.weight"
		}
		if err := writeFile(path, name, wd.WeightString()); err != nil {
			return err
		}
		if !s.SupportV2 {
			if err := writeFile(path, "blkio.leaf_weight_device", wd.LeafWeightString()); err != nil {
				return err
			}
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleReadBpsDevice {
		name := "blkio.throttle.read_bps_device"
		readBps := td.String()
		if s.SupportV2 {
			name = "io.max"
			readBps = fmt.Sprintf("%d:%d rbps=%d", td.Major, td.Minor, td.Rate)
		}
		if err := writeFile(path, name, readBps); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleWriteBpsDevice {
		name := "blkio.throttle.write_bps_device"
		writeBps := td.String()
		if s.SupportV2 {
			name = "io.max"
			writeBps = fmt.Sprintf("%d:%d wbps=%d", td.Major, td.Minor, td.Rate)
		}
		if err := writeFile(path, name, writeBps); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleReadIOPSDevice {
		name := "blkio.throttle.read_iops_device"
		readIops := td.String()
		if s.SupportV2 {
			name = "io.max"
			readIops = fmt.Sprintf("%d:%d riops=%d", td.Major, td.Minor, td.Rate)
		}
		if err := writeFile(path, name, readIops); err != nil {
			return err
		}
	}

	for _, td := range cgroup.Resources.BlkioThrottleWriteIOPSDevice {
		name := "blkio.throttle.write_iops_device"
		writeIops := td.String()
		if s.SupportV2 {
			name = "io.max"
			writeIops = fmt.Sprintf("%d:%d wiops=%d", td.Major, td.Minor, td.Rate)
		}
		if err := writeFile(path, name, writeIops); err != nil {
			return err
		}
	}

	return nil
}

func (s *BlkioGroup) Remove(d *cgroupData) error {
	path, err := d.path("blkio")
	if cgroups.IsV2Error(err) {
		err = nil
	}
	return removePath(path, err)
}

/*
examples:

    blkio.sectors
    8:0 6792

    blkio.io_service_bytes
    8:0 Read 1282048
    8:0 Write 2195456
    8:0 Sync 2195456
    8:0 Async 1282048
    8:0 Total 3477504
    Total 3477504

    blkio.io_serviced
    8:0 Read 124
    8:0 Write 104
    8:0 Sync 104
    8:0 Async 124
    8:0 Total 228
    Total 228

    blkio.io_queued
    8:0 Read 0
    8:0 Write 0
    8:0 Sync 0
    8:0 Async 0
    8:0 Total 0
    Total 0
*/

func splitBlkioStatLine(r rune) bool {
	return r == ' ' || r == ':'
}

func getBlkioStat(path string) ([]cgroups.BlkioStatEntry, error) {
	var blkioStats []cgroups.BlkioStatEntry
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return blkioStats, nil
		}
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// format: dev type amount
		fields := strings.FieldsFunc(sc.Text(), splitBlkioStatLine)
		if len(fields) < 3 {
			if len(fields) == 2 && fields[0] == "Total" {
				// skip total line
				continue
			} else {
				return nil, fmt.Errorf("Invalid line found while parsing %s: %s", path, sc.Text())
			}
		}

		v, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, err
		}
		major := v

		v, err = strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}
		minor := v

		op := ""
		valueField := 2
		if len(fields) == 4 {
			op = fields[2]
			valueField = 3
		}
		v, err = strconv.ParseUint(fields[valueField], 10, 64)
		if err != nil {
			return nil, err
		}
		blkioStats = append(blkioStats, cgroups.BlkioStatEntry{Major: major, Minor: minor, Op: op, Value: v})
	}

	return blkioStats, nil
}

func (s *BlkioGroup) GetStats(path string, stats *cgroups.Stats) error {
	if s.SupportV2 {
		return nil
	}
	// Try to read CFQ stats available on all CFQ enabled kernels first
	if blkioStats, err := getBlkioStat(filepath.Join(path, "blkio.io_serviced_recursive")); err == nil && blkioStats != nil {
		return getCFQStats(path, stats)
	}
	return getStats(path, stats) // Use generic stats as fallback
}

func getCFQStats(path string, stats *cgroups.Stats) error {

	var blkioStats []cgroups.BlkioStatEntry
	var err error

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.sectors_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.SectorsRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_service_bytes_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoServiceBytesRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_serviced_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoServicedRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_queued_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoQueuedRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_service_time_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoServiceTimeRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_wait_time_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoWaitTimeRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.io_merged_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoMergedRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.time_recursive")); err != nil {
		return err
	}
	stats.BlkioStats.IoTimeRecursive = blkioStats

	return nil
}

func getStats(path string, stats *cgroups.Stats) error {
	var blkioStats []cgroups.BlkioStatEntry
	var err error

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.throttle.io_service_bytes")); err != nil {
		return err
	}
	stats.BlkioStats.IoServiceBytesRecursive = blkioStats

	if blkioStats, err = getBlkioStat(filepath.Join(path, "blkio.throttle.io_serviced")); err != nil {
		return err
	}
	stats.BlkioStats.IoServicedRecursive = blkioStats

	return nil
}

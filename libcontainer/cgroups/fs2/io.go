// +build linux

package fs2

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func isIoSet(r *configs.Resources) bool {
	return r.BlkioWeight != 0 ||
		len(r.BlkioThrottleReadBpsDevice) > 0 ||
		len(r.BlkioThrottleWriteBpsDevice) > 0 ||
		len(r.BlkioThrottleReadIOPSDevice) > 0 ||
		len(r.BlkioThrottleWriteIOPSDevice) > 0
}

func setIo(dirPath string, r *configs.Resources) error {
	if !isIoSet(r) {
		return nil
	}

	if r.BlkioWeight != 0 {
		filename := "io.bfq.weight"
		if err := fscommon.WriteFile(dirPath, filename,
			strconv.FormatUint(uint64(r.BlkioWeight), 10)); err != nil {
			// if io.bfq.weight does not exist, then bfq module is not loaded.
			// Fallback to use io.weight with a conversion scheme
			if !os.IsNotExist(err) {
				return err
			}
			v := cgroups.ConvertBlkIOToIOWeightValue(r.BlkioWeight)
			if err := fscommon.WriteFile(dirPath, "io.weight", strconv.FormatUint(v, 10)); err != nil {
				return err
			}
		}
	}
	for _, td := range r.BlkioThrottleReadBpsDevice {
		if err := fscommon.WriteFile(dirPath, "io.max", td.StringName("rbps")); err != nil {
			return err
		}
	}
	for _, td := range r.BlkioThrottleWriteBpsDevice {
		if err := fscommon.WriteFile(dirPath, "io.max", td.StringName("wbps")); err != nil {
			return err
		}
	}
	for _, td := range r.BlkioThrottleReadIOPSDevice {
		if err := fscommon.WriteFile(dirPath, "io.max", td.StringName("riops")); err != nil {
			return err
		}
	}
	for _, td := range r.BlkioThrottleWriteIOPSDevice {
		if err := fscommon.WriteFile(dirPath, "io.max", td.StringName("wiops")); err != nil {
			return err
		}
	}

	return nil
}

func readCgroup2MapFile(dirPath string, name string) (map[string][]string, error) {
	ret := map[string][]string{}
	f, err := fscommon.OpenFile(dirPath, name, os.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ret[parts[0]] = parts[1:]
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func statIo(dirPath string, stats *cgroups.Stats) error {
	values, err := readCgroup2MapFile(dirPath, "io.stat")
	if err != nil {
		return err
	}
	// more details on the io.stat file format: https://www.kernel.org/doc/Documentation/cgroup-v2.txt
	var parsedStats cgroups.BlkioStats
	for k, v := range values {
		d := strings.Split(k, ":")
		if len(d) != 2 {
			continue
		}
		major, err := strconv.ParseUint(d[0], 10, 64)
		if err != nil {
			return err
		}
		minor, err := strconv.ParseUint(d[1], 10, 64)
		if err != nil {
			return err
		}

		for _, item := range v {
			d := strings.Split(item, "=")
			if len(d) != 2 {
				continue
			}
			op := d[0]

			// Map to the cgroupv1 naming and layout (in separate tables).
			var targetTable *[]cgroups.BlkioStatEntry
			switch op {
			// Equivalent to cgroupv1's blkio.io_service_bytes.
			case "rbytes":
				op = "Read"
				targetTable = &parsedStats.IoServiceBytesRecursive
			case "wbytes":
				op = "Write"
				targetTable = &parsedStats.IoServiceBytesRecursive
			// Equivalent to cgroupv1's blkio.io_serviced.
			case "rios":
				op = "Read"
				targetTable = &parsedStats.IoServicedRecursive
			case "wios":
				op = "Write"
				targetTable = &parsedStats.IoServicedRecursive
			default:
				// Skip over entries we cannot map to cgroupv1 stats for now.
				// In the future we should expand the stats struct to include
				// them.
				logrus.Debugf("cgroupv2 io stats: skipping over unmappable %s entry", item)
				continue
			}

			value, err := strconv.ParseUint(d[1], 10, 64)
			if err != nil {
				return err
			}

			entry := cgroups.BlkioStatEntry{
				Op:    op,
				Major: major,
				Minor: minor,
				Value: value,
			}
			*targetTable = append(*targetTable, entry)
		}
	}
	stats.BlkioStats = parsedStats
	return nil
}

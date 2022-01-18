package fs2

import (
	"bufio"
	"os"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func isCPUSet(r *configs.Resources) bool {
	return r.CPUWeight != 0 || r.CPUQuota != 0 || r.CPUPeriod != 0
}

func setCPU(dirPath string, r *configs.Resources) error {
	if !isCPUSet(r) {
		return nil
	}

	// NOTE: .CpuShares is not used here. Conversion is the caller's responsibility.
	if r.CPUWeight != 0 {
		if err := cgroups.WriteFile(dirPath, "cpu.weight", strconv.FormatUint(r.CPUWeight, 10)); err != nil {
			return err
		}
	}

	if r.CPUQuota != 0 || r.CPUPeriod != 0 {
		str := "max"
		if r.CPUQuota > 0 {
			str = strconv.FormatInt(r.CPUQuota, 10)
		}
		period := r.CPUPeriod
		if period == 0 {
			// This default value is documented in
			// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html
			period = 100000
		}
		str += " " + strconv.FormatUint(period, 10)
		if err := cgroups.WriteFile(dirPath, "cpu.max", str); err != nil {
			return err
		}
	}

	return nil
}

func statCPU(dirPath string, stats *cgroups.Stats) error {
	const file = "cpu.stat"
	f, err := cgroups.OpenFile(dirPath, file, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			return &parseError{Path: dirPath, File: file, Err: err}
		}
		switch t {
		case "usage_usec":
			stats.CPUStats.CPUUsage.TotalUsage = v * 1000

		case "user_usec":
			stats.CPUStats.CPUUsage.UsageInUsermode = v * 1000

		case "system_usec":
			stats.CPUStats.CPUUsage.UsageInKernelmode = v * 1000

		case "nr_periods":
			stats.CPUStats.ThrottlingData.Periods = v

		case "nr_throttled":
			stats.CPUStats.ThrottlingData.ThrottledPeriods = v

		case "throttled_usec":
			stats.CPUStats.ThrottlingData.ThrottledTime = v * 1000
		}
	}
	if err := sc.Err(); err != nil {
		return &parseError{Path: dirPath, File: file, Err: err}
	}
	return nil
}

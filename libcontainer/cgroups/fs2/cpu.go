package fs2

import (
	"bufio"
	"errors"
	"os"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

func isCpuSet(r *configs.Resources) bool {
	return r.CpuWeight != 0 || r.CpuQuota != 0 || r.CpuPeriod != 0 || r.CPUIdle != 0
}

func setCpu(dirPath string, r *configs.Resources) error {
	// Since cpu.idle is introduced in Linux 5.15, the default value is 0,
	// and we don't have a way to see if CpuIdle is set from the OCI spec
	// or not, do always write 0 (to reset it to the default).
	if err := cgroups.WriteFile(dirPath, "cpu.idle", strconv.FormatUint(uint64(r.CPUIdle), 10)); err != nil {
		// Ignore ENOENT for the default value, assuming if the file
		// is not present (old kernel), the behavior is the same
		// as when cpu.idle == 0.
		if !(r.CPUIdle == 0 && errors.Is(err, unix.ENOENT)) {
			return err
		}
	}

	if !isCpuSet(r) {
		return nil
	}

	// NOTE: .CpuShares is not used here. Conversion is the caller's responsibility.
	if r.CpuWeight != 0 {
		if err := cgroups.WriteFile(dirPath, "cpu.weight", strconv.FormatUint(r.CpuWeight, 10)); err != nil {
			return err
		}
	}

	if r.CpuQuota != 0 || r.CpuPeriod != 0 {
		str := "max"
		if r.CpuQuota > 0 {
			str = strconv.FormatInt(r.CpuQuota, 10)
		}
		period := r.CpuPeriod
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

func statCpu(dirPath string, stats *cgroups.Stats) error {
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
			stats.CpuStats.CpuUsage.TotalUsage = v * 1000

		case "user_usec":
			stats.CpuStats.CpuUsage.UsageInUsermode = v * 1000

		case "system_usec":
			stats.CpuStats.CpuUsage.UsageInKernelmode = v * 1000

		case "nr_periods":
			stats.CpuStats.ThrottlingData.Periods = v

		case "nr_throttled":
			stats.CpuStats.ThrottlingData.ThrottledPeriods = v

		case "throttled_usec":
			stats.CpuStats.ThrottlingData.ThrottledTime = v * 1000
		}
	}
	if err := sc.Err(); err != nil {
		return &parseError{Path: dirPath, File: file, Err: err}
	}
	return nil
}

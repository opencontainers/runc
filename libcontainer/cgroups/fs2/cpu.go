// +build linux

package fs2

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/pkg/errors"
)

func isCpuSet(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.CpuWeight != 0 || cgroup.Resources.CpuQuota != 0 || cgroup.Resources.CpuPeriod != 0
}

func setCpu(dirPath string, cgroup *configs.Cgroup) error {
	if !isCpuSet(cgroup) {
		return nil
	}

	// NOTE: .CpuShares is not used here. Conversion is the caller's responsibility.
	if cgroup.Resources.CpuWeight != 0 {
		if err := fscommon.WriteFile(dirPath, "cpu.weight", strconv.FormatUint(cgroup.Resources.CpuWeight, 10)); err != nil {
			return err
		}
	}

	// Convert .CpuQuota and .CpuPeriod into "cpu.max".
	// Conversion requires the previous value of "cpu.max"
	prevCpuMaxBytes, err := ioutil.ReadFile(filepath.Join(dirPath, "cpu.max"))
	if err != nil {
		return err
	}
	cpuMax, err := ConvertCPUQuotaCPUPeriodToCgroupV2Value(
		strings.TrimSpace(string(prevCpuMaxBytes)),
		cgroup.Resources.CpuQuota, cgroup.Resources.CpuPeriod)
	if err != nil {
		return err
	}
	if cpuMax != "" {
		if err := fscommon.WriteFile(dirPath, "cpu.max", cpuMax); err != nil {
			return err
		}
	}
	return nil
}

// ConvertCPUQuotaCPUPeriodToCgroupV2Value generates cpu.max string.
func ConvertCPUQuotaCPUPeriodToCgroupV2Value(prevCpuMax string, quota int64, period uint64) (string, error) {
	if quota == 0 && period == 0 {
		return prevCpuMax, nil
	}

	prevPair := strings.Fields(prevCpuMax)
	if len(prevPair) < 2 {
		return "", errors.Errorf("bad cpu.max: %q", prevCpuMax)
	}
	// prevQuotaStr is either a positive decimal integer or "max"
	prevQuotaStr := prevPair[0]
	prevPeriodStr := prevPair[1]
	prevPeriod, err := strconv.Atoi(prevPeriodStr)
	if err != nil {
		return "", errors.Errorf("bad cpu.max: %q", prevCpuMax)
	}

	if period == 0 {
		period = uint64(prevPeriod)
	}
	if quota < 0 {
		return fmt.Sprintf("max %d", period), nil
	}
	if quota == 0 {
		return fmt.Sprintf("%s %d", prevQuotaStr, period), nil
	}
	return fmt.Sprintf("%d %d", quota, period), nil
}

func statCpu(dirPath string, stats *cgroups.Stats) error {
	f, err := os.Open(filepath.Join(dirPath, "cpu.stat"))
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := fscommon.GetCgroupParamKeyValue(sc.Text())
		if err != nil {
			return err
		}
		switch t {
		case "usage_usec":
			stats.CpuStats.CpuUsage.TotalUsage = v * 1000

		case "user_usec":
			stats.CpuStats.CpuUsage.UsageInUsermode = v * 1000

		case "system_usec":
			stats.CpuStats.CpuUsage.UsageInKernelmode = v * 1000
		}
	}
	return nil
}

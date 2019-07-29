// +build linux

package fs

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
)

const (
	cgroupCpuacctStat   = "cpuacct.stat"
	nanosecondsInSecond = 1000000000
)

var clockTicks = uint64(system.GetClockTicks())

type CpuacctGroup struct {
}

func (s *CpuacctGroup) Name() string {
	return "cpuacct"
}

func (s *CpuacctGroup) Apply(d *cgroupData) error {
	// we just want to join this group even though we don't set anything
	if _, err := d.join("cpuacct"); err != nil && !cgroups.IsNotFound(err) {
		return err
	}

	return nil
}

func (s *CpuacctGroup) Set(path string, cgroup *configs.Cgroup) error {
	return nil
}

func (s *CpuacctGroup) Remove(d *cgroupData) error {
	return removePath(d.path("cpuacct"))
}

func (s *CpuacctGroup) GetStats(path string, stats *cgroups.Stats) error {
	userModeUsage, kernelModeUsage, err := getCpuUsageBreakdown(path)
	if err != nil {
		return err
	}

	totalUsage, err := getCgroupParamUint(path, "cpuacct.usage")
	if err != nil {
		return err
	}

	percpuUsage, err := getPercpuUsage(path)
	if err != nil {
		return err
	}

	stats.CpuStats.CpuUsage.TotalUsage = totalUsage
	stats.CpuStats.CpuUsage.PercpuUsage = percpuUsage
	stats.CpuStats.CpuUsage.UsageInUsermode = userModeUsage
	stats.CpuStats.CpuUsage.UsageInKernelmode = kernelModeUsage
	return nil
}

// Returns user and kernel usage breakdown in nanoseconds.
func getCpuUsageBreakdown(path string) (uint64, uint64, error) {
	userModeUsage := uint64(0)
	kernelModeUsage := uint64(0)
	const (
		userField   = "user"
		systemField = "system"
	)

	// Expected format:
	// user <usage in ticks>
	// system <usage in ticks>
	data, err := ioutil.ReadFile(filepath.Join(path, cgroupCpuacctStat))
	if err != nil {
		return 0, 0, err
	}
	
	var hasUser = false;
	var hasSystem = false;

	lines := strings.Split(string(data), "\n");
	for _, line := range lines {
		kv := strings.Split(line, " ");
		if (len(kv) != 2) {
			continue;
		}

		if kv[0] == userField {
			hasUser = true;
			if userModeUsage, err = strconv.ParseUint(kv[1], 10, 64); err != nil {
				return 0, 0, err
			}
		} else if kv[0] == systemField {
			hasSystem = true;
			if kernelModeUsage, err = strconv.ParseUint(kv[1], 10, 64); err != nil {
				return 0, 0, err
			}
		}
	}

	if (!hasUser) {
		return 0, 0, fmt.Errorf("field %q is missing in %q", userField, cgroupCpuacctStat)
	}

	if (!hasSystem) {
		return 0, 0, fmt.Errorf("field %q is missing in %q", systemField, cgroupCpuacctStat)
	}

	return (userModeUsage * nanosecondsInSecond) / clockTicks, (kernelModeUsage * nanosecondsInSecond) / clockTicks, nil
}

func getPercpuUsage(path string) ([]uint64, error) {
	percpuUsage := []uint64{}
	data, err := ioutil.ReadFile(filepath.Join(path, "cpuacct.usage_percpu"))
	if err != nil {
		return percpuUsage, err
	}
	for _, value := range strings.Fields(string(data)) {
		value, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return percpuUsage, fmt.Errorf("Unable to convert param value to uint64: %s", err)
		}
		percpuUsage = append(percpuUsage, value)
	}
	return percpuUsage, nil
}

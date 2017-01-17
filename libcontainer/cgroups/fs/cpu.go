// +build linux

package fs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// CPUGroup represents cpu control group.
type CPUGroup struct {
}

// Name returns the subsystem name of the cgroup.
func (s *CPUGroup) Name() string {
	return "cpu"
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *CPUGroup) Apply(d *cgroupData) error {
	// We always want to join the cpu group, to allow fair cpu scheduling
	// on a container basis
	path, err := d.path("cpu")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return s.ApplyDir(path, d.config, d.pid)
}

// ApplyDir moves the process to the cgroup represented by the directory name.
func (s *CPUGroup) ApplyDir(path string, cgroup *configs.Cgroup, pid int) error {
	// This might happen if we have no cpu cgroup mounted.
	// Just do nothing and don't fail.
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	// We should set the real-Time group scheduling settings before moving
	// in the process because if the process is already in SCHED_RR mode
	// and no RT bandwidth is set, adding it will fail.
	if err := setRtSched(path, cgroup); err != nil {
		return err
	}
	// because we are not using d.join we need to place the pid into the procs file
	// unlike the other subsystems
	if err := cgroups.WriteCgroupProc(path, pid); err != nil {
		return err
	}

	return nil
}

func setRtSched(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.CpuRtPeriod != 0 {
		if err := writeFile(path, "cpu.rt_period_us", strconv.FormatInt(cgroup.Resources.CpuRtPeriod, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.CpuRtRuntime != 0 {
		if err := writeFile(path, "cpu.rt_runtime_us", strconv.FormatInt(cgroup.Resources.CpuRtRuntime, 10)); err != nil {
			return err
		}
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *CPUGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.CpuShares != 0 {
		if err := writeFile(path, "cpu.shares", strconv.FormatInt(cgroup.Resources.CpuShares, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.CpuPeriod != 0 {
		if err := writeFile(path, "cpu.cfs_period_us", strconv.FormatInt(cgroup.Resources.CpuPeriod, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.CpuQuota != 0 {
		if err := writeFile(path, "cpu.cfs_quota_us", strconv.FormatInt(cgroup.Resources.CpuQuota, 10)); err != nil {
			return err
		}
	}
	if err := setRtSched(path, cgroup); err != nil {
		return err
	}

	return nil
}

// Remove deletes the cgroup.
func (s *CPUGroup) Remove(d *cgroupData) error {
	return removePath(d.path("cpu"))
}

// GetStats returns the statistic of the cgroup.
func (s *CPUGroup) GetStats(path string, stats *cgroups.Stats) error {
	f, err := os.Open(filepath.Join(path, "cpu.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return err
		}
		switch t {
		case "nr_periods":
			stats.CPUStats.ThrottlingData.Periods = v

		case "nr_throttled":
			stats.CPUStats.ThrottlingData.ThrottledPeriods = v

		case "throttled_time":
			stats.CPUStats.ThrottlingData.ThrottledTime = v
		}
	}
	return nil
}

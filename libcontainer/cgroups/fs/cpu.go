package fs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

type CPUGroup struct{}

func (s *CPUGroup) Name() string {
	return "cpu"
}

func (s *CPUGroup) Apply(path string, r *configs.Resources, pid int) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	// We should set the real-Time group scheduling settings before moving
	// in the process because if the process is already in SCHED_RR mode
	// and no RT bandwidth is set, adding it will fail.
	if err := s.SetRtSched(path, r); err != nil {
		return err
	}
	// Since we are not using apply(), we need to place the pid
	// into the procs file.
	return cgroups.WriteCgroupProc(path, pid)
}

func (s *CPUGroup) SetRtSched(path string, r *configs.Resources) error {
	if r.CPURtPeriod != 0 {
		if err := cgroups.WriteFile(path, "cpu.rt_period_us", strconv.FormatUint(r.CPURtPeriod, 10)); err != nil {
			return err
		}
	}
	if r.CPURtRuntime != 0 {
		if err := cgroups.WriteFile(path, "cpu.rt_runtime_us", strconv.FormatInt(r.CPURtRuntime, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (s *CPUGroup) Set(path string, r *configs.Resources) error {
	if r.CPUShares != 0 {
		shares := r.CPUShares
		if err := cgroups.WriteFile(path, "cpu.shares", strconv.FormatUint(shares, 10)); err != nil {
			return err
		}
		// read it back
		sharesRead, err := fscommon.GetCgroupParamUint(path, "cpu.shares")
		if err != nil {
			return err
		}
		// ... and check
		if shares > sharesRead {
			return fmt.Errorf("the maximum allowed cpu-shares is %d", sharesRead)
		} else if shares < sharesRead {
			return fmt.Errorf("the minimum allowed cpu-shares is %d", sharesRead)
		}
	}

	var period string
	if r.CPUPeriod != 0 {
		period = strconv.FormatUint(r.CPUPeriod, 10)
		if err := cgroups.WriteFile(path, "cpu.cfs_period_us", period); err != nil {
			// Sometimes when the period to be set is smaller
			// than the current one, it is rejected by the kernel
			// (EINVAL) as old_quota/new_period exceeds the parent
			// cgroup quota limit. If this happens and the quota is
			// going to be set, ignore the error for now and retry
			// after setting the quota.
			if !errors.Is(err, unix.EINVAL) || r.CPUQuota == 0 {
				return err
			}
		} else {
			period = ""
		}
	}
	if r.CPUQuota != 0 {
		if err := cgroups.WriteFile(path, "cpu.cfs_quota_us", strconv.FormatInt(r.CPUQuota, 10)); err != nil {
			return err
		}
		if period != "" {
			if err := cgroups.WriteFile(path, "cpu.cfs_period_us", period); err != nil {
				return err
			}
		}
	}
	return s.SetRtSched(path, r)
}

func (s *CPUGroup) GetStats(path string, stats *cgroups.Stats) error {
	const file = "cpu.stat"
	f, err := cgroups.OpenFile(path, file, os.O_RDONLY)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		t, v, err := fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			return &parseError{Path: path, File: file, Err: err}
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

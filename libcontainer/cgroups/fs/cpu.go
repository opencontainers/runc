// +build linux

package fs

import (
	"bufio"
	"fmt"
	"os"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type CPUGroup struct{}

func (s *CPUGroup) Name() string {
	return "cpu"
}

func (s *CPUGroup) Apply(path string, d *cgroupData) error {
	// This might happen if we have no cpu cgroup mounted.
	// Just do nothing and don't fail.
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	// We should set the real-Time group scheduling settings before moving
	// in the process because if the process is already in SCHED_RR mode
	// and no RT bandwidth is set, adding it will fail.
	if err := s.SetRtSched(path, d.config.Resources); err != nil {
		return err
	}
	// Since we are not using join(), we need to place the pid
	// into the procs file unlike other subsystems.
	return cgroups.WriteCgroupProc(path, d.pid)
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
	if r.CPUPeriod != 0 {
		if err := cgroups.WriteFile(path, "cpu.cfs_period_us", strconv.FormatUint(r.CPUPeriod, 10)); err != nil {
			return err
		}
	}
	if r.CPUQuota != 0 {
		if err := cgroups.WriteFile(path, "cpu.cfs_quota_us", strconv.FormatInt(r.CPUQuota, 10)); err != nil {
			return err
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

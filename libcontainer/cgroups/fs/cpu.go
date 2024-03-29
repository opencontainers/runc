package fs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

type CpuGroup struct{}

func (s *CpuGroup) Name() string {
	return "cpu"
}

func (s *CpuGroup) Apply(path string, r *configs.Resources, pid int) error {
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

func (s *CpuGroup) SetRtSched(path string, r *configs.Resources) error {
	const (
		filePeriod  = "cpu.rt_period_us"
		fileRuntime = "cpu.rt_runtime_us"
	)

	if r.CpuRtPeriod == 0 && r.CpuRtRuntime == 0 {
		return nil
	}

	if r.CpuRtPeriod != 0 && r.CpuRtRuntime == 0 {
		return cgroups.WriteFile(path, filePeriod, strconv.FormatUint(r.CpuRtPeriod, 10))
	} else if r.CpuRtRuntime != 0 && r.CpuRtPeriod == 0 {
		return cgroups.WriteFile(path, fileRuntime, strconv.FormatInt(r.CpuRtRuntime, 10))
	}

	// When neither CpuRtPeriod nor CpuRtRuntime is equal to 0, let's set them in the correct order.

	sOldPeriod, err := cgroups.ReadFile(path, filePeriod)
	if err != nil {
		return err
	}
	oldPeriod, err := strconv.ParseInt(strings.TrimSpace(sOldPeriod), 10, 64)
	if err != nil {
		return err
	}

	sOldRuntime, err := cgroups.ReadFile(path, fileRuntime)
	if err != nil {
		return err
	}
	oldRuntime, err := strconv.ParseInt(strings.TrimSpace(sOldRuntime), 10, 64)
	if err != nil {
		return err
	}

	/*
		When we set a new rt_period_us, the kernel will determine whether
		the current configuration of new_limit1 = old_quota/new_period
		exceeds the limit. If it exceeds the limit, an error will be reported.
		Maybe it is reasonable to set rt_runtime_us first so that the
		new_limit2 = new_quota/old_period.In the opposite case, if rt_runtime_us
		is set first, new_limit2 may still exceed the limit, but new_limit1
		will be valid.Therefore, new_limit1 and new_limit2 should be calculated
		in advance,and the smaller corresponding setting order should be selected
		to set rt_period_us and rt_runtime_us.
	*/

	// First set cpu.rt_period_us because of (oldRuntime / r.CpuRtPeriod) < (r.CpuRtRuntime / oldPeriod)
	if uint64(oldRuntime*oldPeriod) < (uint64(r.CpuRtRuntime) * r.CpuRtPeriod) {
		if err := cgroups.WriteFile(path, filePeriod, strconv.FormatUint(r.CpuRtPeriod, 10)); err != nil {
			return err
		}
		return cgroups.WriteFile(path, fileRuntime, strconv.FormatInt(r.CpuRtRuntime, 10))
	}
	// First set cpu.rt_runtime_us because of (r.CpuRtRuntime / oldPeriod) < (oldRuntime / r.CpuRtPeriod)
	if err := cgroups.WriteFile(path, fileRuntime, strconv.FormatInt(r.CpuRtRuntime, 10)); err != nil {
		return err
	}
	return cgroups.WriteFile(path, filePeriod, strconv.FormatUint(r.CpuRtPeriod, 10))
}

func (s *CpuGroup) Set(path string, r *configs.Resources) error {
	if r.CpuShares != 0 {
		shares := r.CpuShares
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
	if r.CpuPeriod != 0 {
		period = strconv.FormatUint(r.CpuPeriod, 10)
		if err := cgroups.WriteFile(path, "cpu.cfs_period_us", period); err != nil {
			// Sometimes when the period to be set is smaller
			// than the current one, it is rejected by the kernel
			// (EINVAL) as old_quota/new_period exceeds the parent
			// cgroup quota limit. If this happens and the quota is
			// going to be set, ignore the error for now and retry
			// after setting the quota.
			if !errors.Is(err, unix.EINVAL) || r.CpuQuota == 0 {
				return err
			}
		} else {
			period = ""
		}
	}

	var burst string
	if r.CpuBurst != nil {
		burst = strconv.FormatUint(*r.CpuBurst, 10)
		if err := cgroups.WriteFile(path, "cpu.cfs_burst_us", burst); err != nil {
			if errors.Is(err, unix.ENOENT) {
				// If CPU burst knob is not available (e.g.
				// older kernel), ignore it.
				burst = ""
			} else {
				// Sometimes when the burst to be set is larger
				// than the current one, it is rejected by the kernel
				// (EINVAL) as old_quota/new_burst exceeds the parent
				// cgroup quota limit. If this happens and the quota is
				// going to be set, ignore the error for now and retry
				// after setting the quota.
				if !errors.Is(err, unix.EINVAL) || r.CpuQuota == 0 {
					return err
				}
			}
		} else {
			burst = ""
		}
	}
	if r.CpuQuota != 0 {
		if err := cgroups.WriteFile(path, "cpu.cfs_quota_us", strconv.FormatInt(r.CpuQuota, 10)); err != nil {
			return err
		}
		if period != "" {
			if err := cgroups.WriteFile(path, "cpu.cfs_period_us", period); err != nil {
				return err
			}
		}
		if burst != "" {
			if err := cgroups.WriteFile(path, "cpu.cfs_burst_us", burst); err != nil {
				return err
			}
		}
	}

	if r.CPUIdle != nil {
		idle := strconv.FormatInt(*r.CPUIdle, 10)
		if err := cgroups.WriteFile(path, "cpu.idle", idle); err != nil {
			return err
		}
	}

	return s.SetRtSched(path, r)
}

func (s *CpuGroup) GetStats(path string, stats *cgroups.Stats) error {
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
			stats.CpuStats.ThrottlingData.Periods = v

		case "nr_throttled":
			stats.CpuStats.ThrottlingData.ThrottledPeriods = v

		case "throttled_time":
			stats.CpuStats.ThrottlingData.ThrottledTime = v
		}
	}
	return nil
}

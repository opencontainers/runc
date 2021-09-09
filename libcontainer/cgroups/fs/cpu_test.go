package fs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestCpuSetShares(t *testing.T) {
	path := tempDir(t, "cpu")

	const (
		sharesBefore = 1024
		sharesAfter  = 512
	)

	writeFileContents(t, path, map[string]string{
		"cpu.shares": strconv.Itoa(sharesBefore),
	})

	r := &configs.Resources{
		CpuShares: sharesAfter,
	}
	cpu := &CpuGroup{}
	if err := cpu.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamUint(path, "cpu.shares")
	if err != nil {
		t.Fatal(err)
	}
	if value != sharesAfter {
		t.Fatal("Got the wrong value, set cpu.shares failed.")
	}
}

func TestCpuSetBandWidth(t *testing.T) {
	path := tempDir(t, "cpu")

	const (
		quotaBefore     = 8000
		quotaAfter      = 5000
		burstBefore     = 2000
		periodBefore    = 10000
		periodAfter     = 7000
		rtRuntimeBefore = 8000
		rtRuntimeAfter  = 5000
		rtPeriodBefore  = 10000
		rtPeriodAfter   = 7000
	)
	burstAfter := uint64(1000)

	writeFileContents(t, path, map[string]string{
		"cpu.cfs_quota_us":  strconv.Itoa(quotaBefore),
		"cpu.cfs_burst_us":  strconv.Itoa(burstBefore),
		"cpu.cfs_period_us": strconv.Itoa(periodBefore),
		"cpu.rt_runtime_us": strconv.Itoa(rtRuntimeBefore),
		"cpu.rt_period_us":  strconv.Itoa(rtPeriodBefore),
	})

	r := &configs.Resources{
		CpuQuota:     quotaAfter,
		CpuBurst:     &burstAfter,
		CpuPeriod:    periodAfter,
		CpuRtRuntime: rtRuntimeAfter,
		CpuRtPeriod:  rtPeriodAfter,
	}
	cpu := &CpuGroup{}
	if err := cpu.Set(path, r); err != nil {
		t.Fatal(err)
	}

	quota, err := fscommon.GetCgroupParamUint(path, "cpu.cfs_quota_us")
	if err != nil {
		t.Fatal(err)
	}
	if quota != quotaAfter {
		t.Fatal("Got the wrong value, set cpu.cfs_quota_us failed.")
	}

	burst, err := fscommon.GetCgroupParamUint(path, "cpu.cfs_burst_us")
	if err != nil {
		t.Fatal(err)
	}
	if burst != burstAfter {
		t.Fatal("Got the wrong value, set cpu.cfs_burst_us failed.")
	}

	period, err := fscommon.GetCgroupParamUint(path, "cpu.cfs_period_us")
	if err != nil {
		t.Fatal(err)
	}
	if period != periodAfter {
		t.Fatal("Got the wrong value, set cpu.cfs_period_us failed.")
	}

	rtRuntime, err := fscommon.GetCgroupParamUint(path, "cpu.rt_runtime_us")
	if err != nil {
		t.Fatal(err)
	}
	if rtRuntime != rtRuntimeAfter {
		t.Fatal("Got the wrong value, set cpu.rt_runtime_us failed.")
	}

	rtPeriod, err := fscommon.GetCgroupParamUint(path, "cpu.rt_period_us")
	if err != nil {
		t.Fatal(err)
	}
	if rtPeriod != rtPeriodAfter {
		t.Fatal("Got the wrong value, set cpu.rt_period_us failed.")
	}
}

func TestCpuStats(t *testing.T) {
	path := tempDir(t, "cpu")

	const (
		nrPeriods     = 2000
		nrThrottled   = 200
		throttledTime = uint64(18446744073709551615)
	)

	cpuStatContent := fmt.Sprintf("nr_periods %d\nnr_throttled %d\nthrottled_time %d\n",
		nrPeriods, nrThrottled, throttledTime)
	writeFileContents(t, path, map[string]string{
		"cpu.stat": cpuStatContent,
	})

	cpu := &CpuGroup{}
	actualStats := *cgroups.NewStats()
	err := cpu.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.ThrottlingData{
		Periods:          nrPeriods,
		ThrottledPeriods: nrThrottled,
		ThrottledTime:    throttledTime,
	}

	expectThrottlingDataEquals(t, expectedStats, actualStats.CpuStats.ThrottlingData)
}

func TestNoCpuStatFile(t *testing.T) {
	path := tempDir(t, "cpu")

	cpu := &CpuGroup{}
	actualStats := *cgroups.NewStats()
	err := cpu.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal("Expected not to fail, but did")
	}
}

func TestInvalidCpuStat(t *testing.T) {
	path := tempDir(t, "cpu")

	cpuStatContent := `nr_periods 2000
	nr_throttled 200
	throttled_time fortytwo`
	writeFileContents(t, path, map[string]string{
		"cpu.stat": cpuStatContent,
	})

	cpu := &CpuGroup{}
	actualStats := *cgroups.NewStats()
	err := cpu.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failed stat parsing.")
	}
}

func TestCpuSetRtSchedAtApply(t *testing.T) {
	path := tempDir(t, "cpu")

	const (
		rtRuntimeBefore = 0
		rtRuntimeAfter  = 5000
		rtPeriodBefore  = 0
		rtPeriodAfter   = 7000
	)

	writeFileContents(t, path, map[string]string{
		"cpu.rt_runtime_us": strconv.Itoa(rtRuntimeBefore),
		"cpu.rt_period_us":  strconv.Itoa(rtPeriodBefore),
	})

	r := &configs.Resources{
		CpuRtRuntime: rtRuntimeAfter,
		CpuRtPeriod:  rtPeriodAfter,
	}
	cpu := &CpuGroup{}

	if err := cpu.Apply(path, r, 1234); err != nil {
		t.Fatal(err)
	}

	rtRuntime, err := fscommon.GetCgroupParamUint(path, "cpu.rt_runtime_us")
	if err != nil {
		t.Fatal(err)
	}
	if rtRuntime != rtRuntimeAfter {
		t.Fatal("Got the wrong value, set cpu.rt_runtime_us failed.")
	}

	rtPeriod, err := fscommon.GetCgroupParamUint(path, "cpu.rt_period_us")
	if err != nil {
		t.Fatal(err)
	}
	if rtPeriod != rtPeriodAfter {
		t.Fatal("Got the wrong value, set cpu.rt_period_us failed.")
	}

	pid, err := fscommon.GetCgroupParamUint(path, "cgroup.procs")
	if err != nil {
		t.Fatal(err)
	}
	if pid != 1234 {
		t.Fatal("Got the wrong value, set cgroup.procs failed.")
	}
}

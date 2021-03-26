// +build linux

package fs2

import (
	"bufio"
	"math"
	"os"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/pkg/errors"
)

// numToStr converts an int64 value to a string for writing to a
// cgroupv2 files with .min, .max, .low, or .high suffix.
// The value of -1 is converted to "max" for cgroupv1 compatibility
// (which used to write -1 to remove the limit).
func numToStr(value int64) (ret string) {
	switch {
	case value == 0:
		ret = ""
	case value == -1:
		ret = "max"
	default:
		ret = strconv.FormatInt(value, 10)
	}

	return ret
}

func isMemorySet(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.MemoryReservation != 0 ||
		cgroup.Resources.Memory != 0 || cgroup.Resources.MemorySwap != 0
}

func setMemory(dirPath string, cgroup *configs.Cgroup) error {
	if !isMemorySet(cgroup) {
		return nil
	}
	swap, err := cgroups.ConvertMemorySwapToCgroupV2Value(cgroup.Resources.MemorySwap, cgroup.Resources.Memory)
	if err != nil {
		return err
	}
	swapStr := numToStr(swap)
	if swapStr == "" && swap == 0 && cgroup.Resources.MemorySwap > 0 {
		// memory and memorySwap set to the same value -- disable swap
		swapStr = "0"
	}
	// never write empty string to `memory.swap.max`, it means set to 0.
	if swapStr != "" {
		if err := fscommon.WriteFile(dirPath, "memory.swap.max", swapStr); err != nil {
			return err
		}
	}

	if val := numToStr(cgroup.Resources.Memory); val != "" {
		if err := fscommon.WriteFile(dirPath, "memory.max", val); err != nil {
			return err
		}
	}

	// cgroup.Resources.KernelMemory is ignored

	if val := numToStr(cgroup.Resources.MemoryReservation); val != "" {
		if err := fscommon.WriteFile(dirPath, "memory.low", val); err != nil {
			return err
		}
	}

	return nil
}

func statMemory(dirPath string, stats *cgroups.Stats) error {
	// Set stats from memory.stat.
	statsFile, err := fscommon.OpenFile(dirPath, "memory.stat", os.O_RDONLY)
	if err != nil {
		return err
	}
	defer statsFile.Close()

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			return errors.Wrapf(err, "failed to parse memory.stat (%q)", sc.Text())
		}
		stats.MemoryStats.Stats[t] = v
	}
	stats.MemoryStats.Cache = stats.MemoryStats.Stats["file"]
	// Unlike cgroup v1 which has memory.use_hierarchy binary knob,
	// cgroup v2 is always hierarchical.
	stats.MemoryStats.UseHierarchy = true

	memoryUsage, err := getMemoryDataV2(dirPath, "")
	if err != nil {
		return err
	}
	stats.MemoryStats.Usage = memoryUsage
	swapUsage, err := getMemoryDataV2(dirPath, "swap")
	if err != nil {
		return err
	}
	// As cgroup v1 reports SwapUsage values as mem+swap combined,
	// while in cgroup v2 swap values do not include memory,
	// report combined mem+swap for v1 compatibility.
	swapUsage.Usage += memoryUsage.Usage
	if swapUsage.Limit != math.MaxUint64 {
		swapUsage.Limit += memoryUsage.Limit
	}
	stats.MemoryStats.SwapUsage = swapUsage

	return nil
}

func getMemoryDataV2(path, name string) (cgroups.MemoryData, error) {
	memoryData := cgroups.MemoryData{}

	moduleName := "memory"
	if name != "" {
		moduleName = "memory." + name
	}
	usage := moduleName + ".current"
	limit := moduleName + ".max"

	value, err := fscommon.GetCgroupParamUint(path, usage)
	if err != nil {
		if name != "" && os.IsNotExist(err) {
			// Ignore EEXIST as there's no swap accounting
			// if kernel CONFIG_MEMCG_SWAP is not set or
			// swapaccount=0 kernel boot parameter is given.
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, errors.Wrapf(err, "failed to parse %s", usage)
	}
	memoryData.Usage = value

	value, err = fscommon.GetCgroupParamUint(path, limit)
	if err != nil {
		return cgroups.MemoryData{}, errors.Wrapf(err, "failed to parse %s", limit)
	}
	memoryData.Limit = value

	return memoryData, nil
}

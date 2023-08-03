package fs2

import (
	"bufio"
	"errors"
	"math"
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
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

func isMemorySet(r *configs.Resources) bool {
	return r.MemoryReservation != 0 || r.Memory != 0 || r.MemorySwap != 0
}

func setMemory(dirPath string, r *configs.Resources) error {
	if !isMemorySet(r) {
		return nil
	}

	if err := CheckMemoryUsage(dirPath, r); err != nil {
		return err
	}

	swap, err := cgroups.ConvertMemorySwapToCgroupV2Value(r.MemorySwap, r.Memory)
	if err != nil {
		return err
	}
	swapStr := numToStr(swap)
	if swapStr == "" && swap == 0 && r.MemorySwap > 0 {
		// memory and memorySwap set to the same value -- disable swap
		swapStr = "0"
	}
	// never write empty string to `memory.swap.max`, it means set to 0.
	if swapStr != "" {
		if err := cgroups.WriteFile(dirPath, "memory.swap.max", swapStr); err != nil {
			return err
		}
	}

	if val := numToStr(r.Memory); val != "" {
		if err := cgroups.WriteFile(dirPath, "memory.max", val); err != nil {
			return err
		}
	}

	// cgroup.Resources.KernelMemory is ignored

	if val := numToStr(r.MemoryReservation); val != "" {
		if err := cgroups.WriteFile(dirPath, "memory.low", val); err != nil {
			return err
		}
	}

	return nil
}

func statMemory(dirPath string, stats *cgroups.Stats) error {
	const file = "memory.stat"
	statsFile, err := cgroups.OpenFile(dirPath, file, os.O_RDONLY)
	if err != nil {
		return err
	}
	defer statsFile.Close()

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := fscommon.ParseKeyValue(sc.Text())
		if err != nil {
			return &parseError{Path: dirPath, File: file, Err: err}
		}
		stats.MemoryStats.Stats[t] = v
	}
	if err := sc.Err(); err != nil {
		return &parseError{Path: dirPath, File: file, Err: err}
	}
	stats.MemoryStats.Cache = stats.MemoryStats.Stats["file"]
	// Unlike cgroup v1 which has memory.use_hierarchy binary knob,
	// cgroup v2 is always hierarchical.
	stats.MemoryStats.UseHierarchy = true

	pagesByNUMA, err := getPageUsageByNUMAV2(dirPath)
	if err != nil {
		return err
	}
	stats.MemoryStats.PageUsageByNUMA = pagesByNUMA

	memoryUsage, err := getMemoryDataV2(dirPath, "")
	if err != nil {
		if errors.Is(err, unix.ENOENT) && dirPath == UnifiedMountpoint {
			// The root cgroup does not have memory.{current,max}
			// so emulate those using data from /proc/meminfo.
			return statsFromMeminfo(stats)
		}
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
	if stats.MemoryStats.PageUsageByNUMA.Hierarchical.Total.Total != 0 {
		stats.MemoryStats.UseHierarchy = true
	}
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
		return cgroups.MemoryData{}, err
	}
	memoryData.Usage = value

	value, err = fscommon.GetCgroupParamUint(path, limit)
	if err != nil {
		return cgroups.MemoryData{}, err
	}
	memoryData.Limit = value

	return memoryData, nil
}

func statsFromMeminfo(stats *cgroups.Stats) error {
	const file = "/proc/meminfo"
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	// Fields we are interested in.
	var (
		swap_free  uint64
		swap_total uint64
		main_total uint64
		main_free  uint64
	)
	mem := map[string]*uint64{
		"SwapFree":  &swap_free,
		"SwapTotal": &swap_total,
		"MemTotal":  &main_total,
		"MemFree":   &main_free,
	}

	found := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), ":", 3)
		if len(parts) != 2 {
			// Should not happen.
			continue
		}
		k := parts[0]
		p, ok := mem[k]
		if !ok {
			// Unknown field -- not interested.
			continue
		}
		vStr := strings.TrimSpace(strings.TrimSuffix(parts[1], " kB"))
		*p, err = strconv.ParseUint(vStr, 10, 64)
		if err != nil {
			return &parseError{File: file, Err: errors.New("bad value for " + k)}
		}

		found++
		if found == len(mem) {
			// Got everything we need -- skip the rest.
			break
		}
	}
	if err := sc.Err(); err != nil {
		return &parseError{Path: "", File: file, Err: err}
	}

	stats.MemoryStats.SwapUsage.Usage = (swap_total - swap_free) * 1024
	stats.MemoryStats.SwapUsage.Limit = math.MaxUint64

	stats.MemoryStats.Usage.Usage = (main_total - main_free) * 1024
	stats.MemoryStats.Usage.Limit = math.MaxUint64

	return nil
}

func getPageUsageByNUMAV2(path string) (cgroups.PageUsageByNUMA, error) {
	const (
		maxColumns = math.MaxUint8 + 1
		file       = "memory.numa_stat"
	)
	stats := cgroups.PageUsageByNUMA{}

	fd, err := cgroups.OpenFile(path, file, os.O_RDONLY)
	if os.IsNotExist(err) {
		return stats, nil
	} else if err != nil {
		return stats, err
	}
	defer fd.Close()

	// https://docs.kernel.org/admin-guide/cgroup-v2.html.
	// anon N0=<> N1=<> # The Anon page size in byte which equals to page_num * page_size.
	// file N0=<> N1=0 # The File page size in byte which equals to file_mmaped_page_num * page_size.
	// kernel_stack N0=<> N1=0 # The Kernel's stack occupation.
	// pagetables N0=<> N1=0 # The total number of pagetable entry been occupied.
	// sec_pagetables N0=<> N1=<>
	// shmem N0=<> N1=<>
	// file_mapped N0=<> N1=<> # file page breakdown.
	// file_dirty N0=<> N1=<> # file page breakdown.
	// file_writeback N0=<> N1=<> # file page breakdown.
	// swapcached N0=<> N1=<>
	// anon_thp N0=<> N1=<> # The transparent huge page occupation.
	// file_thp N0=<> N1=<> # The transparent huge page occupation.
	// shmem_thp N0=<> N1=<> # The transparent huge page occupation.
	// inactive_anon N0=<> N1=<>
	// active_anon N0=<> N1=<>
	// inactive_file N0=<> N1=<>
	// active_file N0=<> N1=<>
	// unevictable N0=<> N1=<>
	// slab_reclaimable N0=<> N1=<>
	// slab_unreclaimable N0=<> N1=<>
	// workingset_refault_anon N0=<> N1=<>
	// workingset_refault_file N0=<> N1=<>
	// workingset_activate_anon N0=<> N1=<>
	// workingset_activate_file N0=<> N1=<>
	// workingset_restore_anon N0=<> N1=<>
	// workingset_restore_file N0=<> N1=<>
	// workingset_nodereclaim N0=<> N1=<>

	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		var field *cgroups.PageStats

		line := scanner.Text()
		columns := strings.SplitN(line, " ", maxColumns)
		for i, column := range columns {
			byNode := strings.SplitN(column, "=", 2)
			key := byNode[0]
			if i == 0 { // First column: key is name, val is total.
				field = getNUMAFieldV2(&stats, key)
				if field == nil { // unknown field (new kernel?)
					break
				}
				field.Nodes = map[uint8]uint64{}
			} else { // Subsequent columns: key is N<id>, val is usage.
				if len(byNode) != 2 {
					// This is definitely an error.
					return stats, malformedLine(path, file, line)
				}
				val := byNode[1]
				if len(key) < 2 || key[0] != 'N' {
					// This is definitely an error.
					return stats, malformedLine(path, file, line)
				}

				n, err := strconv.ParseUint(key[1:], 10, 8)
				if err != nil {
					return stats, &parseError{Path: path, File: file, Err: err}
				}

				usage, err := strconv.ParseUint(val, 10, 64)
				if err != nil {
					return stats, &parseError{Path: path, File: file, Err: err}
				}
				field.Nodes[uint8(n)] += usage
				field.Total += usage
			}

		}
		stats.Total.Total = stats.File.Total + stats.Anon.Total
		stats.Total.Nodes = map[uint8]uint64{}
		for k, v := range stats.File.Nodes {
			stats.Total.Nodes[k] = v + stats.Anon.Nodes[k]
		}
	}
	if err := scanner.Err(); err != nil {
		return cgroups.PageUsageByNUMA{}, &parseError{Path: path, File: file, Err: err}
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return stats, err
	}
	// hierarchical stats in subdirectory
	for _, file := range files {
		if file.IsDir() {
			statTmp, err := getPageUsageByNUMAV2(path + "/" + file.Name())
			if err != nil {
				return stats, err
			}
			if stats.Hierarchical.Total.Total == 0 {
				stats.Hierarchical.Total.Nodes = map[uint8]uint64{}
				stats.Hierarchical.Anon.Nodes = map[uint8]uint64{}
				stats.Hierarchical.File.Nodes = map[uint8]uint64{}
				stats.Hierarchical.Unevictable.Nodes = map[uint8]uint64{}
			}
			stats.Hierarchical.Total.Total += statTmp.Total.Total
			stats.Hierarchical.Anon.Total += statTmp.Anon.Total
			stats.Hierarchical.File.Total += statTmp.File.Total
			stats.Hierarchical.Unevictable.Total += statTmp.Unevictable.Total
			for k, v := range statTmp.Total.Nodes {
				stats.Hierarchical.Total.Nodes[k] += v
			}
			for k, v := range statTmp.Anon.Nodes {
				stats.Hierarchical.Anon.Nodes[k] += v
			}
			for k, v := range statTmp.File.Nodes {
				stats.Hierarchical.File.Nodes[k] += v
			}
			for k, v := range statTmp.Unevictable.Nodes {
				stats.Hierarchical.Unevictable.Nodes[k] += v
			}
		}
	}
	return stats, nil
}

func getNUMAFieldV2(stats *cgroups.PageUsageByNUMA, name string) *cgroups.PageStats {
	switch name {
	case "anon":
		return &stats.Anon
	case "file":
		return &stats.File
	case "unevictable":
		return &stats.Unevictable
	}
	return nil
}

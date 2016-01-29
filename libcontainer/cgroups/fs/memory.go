// +build linux

package fs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type MemoryGroup struct {
}

func (s *MemoryGroup) Name() string {
	return "memory"
}

func (s *MemoryGroup) Apply(d *cgroupData) (err error) {
	path, err := d.path("memory")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	if memoryAssigned(d.config) {
		if path != "" {
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		}
		// We have to set kernel memory here, as we can't change it once
		// processes have been attached.
		if err := s.SetKernelMemory(path, d.config); err != nil {
			return err
		}
	}

	defer func() {
		if err != nil {
			os.RemoveAll(path)
		}
	}()

	// We need to join memory cgroup after set memory limits, because
	// kmem.limit_in_bytes can only be set when the cgroup is empty.
	_, err = d.join("memory")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *MemoryGroup) SetKernelMemory(path string, cgroup *configs.Cgroup) error {
	// This has to be done separately because it has special constraints (it
	// can't be done after there are processes attached to the cgroup).
	if cgroup.Resources.KernelMemory > 0 {
		if err := writeFile(path, "memory.kmem.limit_in_bytes", strconv.FormatInt(cgroup.Resources.KernelMemory, 10)); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemoryGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.Memory != 0 {
		if err := writeFile(path, "memory.limit_in_bytes", strconv.FormatInt(cgroup.Resources.Memory, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemoryReservation != 0 {
		if err := writeFile(path, "memory.soft_limit_in_bytes", strconv.FormatInt(cgroup.Resources.MemoryReservation, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemorySwap > 0 {
		if err := writeFile(path, "memory.memsw.limit_in_bytes", strconv.FormatInt(cgroup.Resources.MemorySwap, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.OomKillDisable {
		if err := writeFile(path, "memory.oom_control", "1"); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemorySwappiness >= 0 && cgroup.Resources.MemorySwappiness <= 100 {
		if err := writeFile(path, "memory.swappiness", strconv.FormatInt(cgroup.Resources.MemorySwappiness, 10)); err != nil {
			return err
		}
	} else if cgroup.Resources.MemorySwappiness == -1 {
		return nil
	} else {
		return fmt.Errorf("invalid value:%d. valid memory swappiness range is 0-100", cgroup.Resources.MemorySwappiness)
	}

	return nil
}

func (s *MemoryGroup) Remove(d *cgroupData) error {
	return removePath(d.path("memory"))
}

func (s *MemoryGroup) GetStats(path string, stats *cgroups.Stats) error {
	memoryStat, err := getMemoryStat(path)
	if err != nil {
		return err
	}
	memoryUsage, err := getMemoryData(path, "")
	if err != nil {
		return err
	}
	swapUsage, err := getMemoryData(path, "memsw")
	if err != nil {
		return err
	}
	kernelUsage, err := getMemoryData(path, "kmem")
	if err != nil {
		return err
	}
	oomStat, err := getOomStat(path)
	if err != nil {
		return err
	}

	stats.MemoryStats = cgroups.MemoryStats{
		Stats:       memoryStat,
		Cache:       memoryStat["cache"],
		Usage:       memoryUsage,
		SwapUsage:   swapUsage,
		KernelUsage: kernelUsage,
		OomStat:     oomStat,
	}

	return nil
}

func memoryAssigned(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.Memory != 0 ||
		cgroup.Resources.MemoryReservation != 0 ||
		cgroup.Resources.MemorySwap > 0 ||
		cgroup.Resources.KernelMemory > 0 ||
		cgroup.Resources.OomKillDisable ||
		cgroup.Resources.MemorySwappiness != -1
}

func getMemoryMappingData(path, name string) (map[string]uint64, error) {
	data := make(map[string]uint64)

	file, err := os.Open(filepath.Join(path, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	sc := bufio.NewScanner(file)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s (%q) - %v", name, sc.Text(), err)
		}
		data[t] = v
	}

	return data, nil
}

func getMemoryStat(path string) (map[string]uint64, error) {
	// Get stats from memory.stat.
	return getMemoryMappingData(path, "memory.stat")
}

func getOomStat(path string) (map[string]uint64, error) {
	// Get oom stats from memory.oom_control.
	return getMemoryMappingData(path, "memory.oom_control")
}

func getMemoryData(path, name string) (cgroups.MemoryData, error) {
	memoryData := cgroups.MemoryData{}

	moduleName := "memory"
	if name != "" {
		moduleName = strings.Join([]string{"memory", name}, ".")
	}
	usage := strings.Join([]string{moduleName, "usage_in_bytes"}, ".")
	maxUsage := strings.Join([]string{moduleName, "max_usage_in_bytes"}, ".")
	failcnt := strings.Join([]string{moduleName, "failcnt"}, ".")

	value, err := getCgroupParamUint(path, usage)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s - %v", usage, err)
	}
	memoryData.Usage = value
	value, err = getCgroupParamUint(path, maxUsage)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s - %v", maxUsage, err)
	}
	memoryData.MaxUsage = value
	value, err = getCgroupParamUint(path, failcnt)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s - %v", failcnt, err)
	}
	memoryData.Failcnt = value

	return memoryData, nil
}

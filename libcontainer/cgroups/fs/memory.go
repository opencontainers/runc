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

		if err := s.Set(path, d.config); err != nil {
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

func (s *MemoryGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.Memory != nil {
		if err := writeFile(path, "memory.limit_in_bytes", strconv.FormatUint(*cgroup.Resources.Memory, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemoryReservation != nil {
		if err := writeFile(path, "memory.soft_limit_in_bytes", strconv.FormatUint(*cgroup.Resources.MemoryReservation, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemorySwap != nil {
		if err := writeFile(path, "memory.memsw.limit_in_bytes", strconv.FormatUint(*cgroup.Resources.MemorySwap, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.KernelMemory != nil {
		if err := writeFile(path, "memory.kmem.limit_in_bytes", strconv.FormatUint(*cgroup.Resources.KernelMemory, 10)); err != nil {
			return err
		}
	}

	if cgroup.Resources.OomKillDisable != nil {
		if *cgroup.Resources.OomKillDisable {
			if err := writeFile(path, "memory.oom_control", "1"); err != nil {
				return err
			}
		}
	}
	if cgroup.Resources.MemorySwappiness != nil {
		if *cgroup.Resources.MemorySwappiness >= 0 && *cgroup.Resources.MemorySwappiness <= 100 {
			if err := writeFile(path, "memory.swappiness", strconv.FormatUint(*cgroup.Resources.MemorySwappiness, 10)); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("invalid value:%d. valid memory swappiness range is 0-100", *cgroup.Resources.MemorySwappiness)
		}
	}

	return nil
}

func (s *MemoryGroup) Remove(d *cgroupData) error {
	return removePath(d.path("memory"))
}

func (s *MemoryGroup) GetStats(path string, stats *cgroups.Stats) error {
	// Set stats from memory.stat.
	statsFile, err := os.Open(filepath.Join(path, "memory.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer statsFile.Close()

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return fmt.Errorf("failed to parse memory.stat (%q) - %v", sc.Text(), err)
		}
		stats.MemoryStats.Stats[t] = v
	}
	stats.MemoryStats.Cache = stats.MemoryStats.Stats["cache"]

	memoryUsage, err := getMemoryData(path, "")
	if err != nil {
		return err
	}
	stats.MemoryStats.Usage = memoryUsage
	swapUsage, err := getMemoryData(path, "memsw")
	if err != nil {
		return err
	}
	stats.MemoryStats.SwapUsage = swapUsage
	kernelUsage, err := getMemoryData(path, "kmem")
	if err != nil {
		return err
	}
	stats.MemoryStats.KernelUsage = kernelUsage

	return nil
}

func memoryAssigned(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.Memory != nil ||
		cgroup.Resources.MemoryReservation != nil ||
		cgroup.Resources.MemorySwap != nil ||
		cgroup.Resources.KernelMemory != nil ||
		cgroup.Resources.OomKillDisable != nil
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

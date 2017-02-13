// +build linux

package fs

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const cgroupKernelMemoryLimit = "memory.kmem.limit_in_bytes"

type MemoryGroup struct {
	SupportV2 bool
}

func (s *MemoryGroup) Name() string {
	return "memory"
}

func (s *MemoryGroup) Apply(d *cgroupData) (err error) {
	path, err := d.path("memory")
	if cgroups.IsV2Error(err) {
		s.SupportV2 = true
	} else if err != nil && !cgroups.IsNotFound(err) {
		return
	}
	if memoryAssigned(d.config) {
		if path != "" {
			if subErr := os.MkdirAll(path, 0755); subErr != nil {
				return subErr
			}
		}
		if cgroups.IsV2Error(err) {
			if subErr := d.addControllerForV2("memory", path); subErr != nil {
				err = subErr
				return
			}
		}
		if d.config.KernelMemory != 0 {
			if err := EnableKernelMemoryAccounting(path); err != nil {
				return err
			}
		}
	}
	defer func() {
		if err != nil && !cgroups.IsV2Error(err) {
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

func EnableKernelMemoryAccounting(path string) error {
	// Check if kernel memory is enabled
	// We have to limit the kernel memory here as it won't be accounted at all
	// until a limit is set on the cgroup and limit cannot be set once the
	// cgroup has children, or if there are already tasks in the cgroup.
	for _, i := range []int64{1, -1} {
		if err := setKernelMemory(path, i); err != nil {
			return err
		}
	}
	return nil
}

func setKernelMemory(path string, kernelMemoryLimit int64) error {
	if path == "" {
		return fmt.Errorf("no such directory for %s", cgroupKernelMemoryLimit)
	}
	if !cgroups.PathExists(filepath.Join(path, cgroupKernelMemoryLimit)) {
		// kernel memory is not enabled on the system so we should do nothing
		return nil
	}
	if err := ioutil.WriteFile(filepath.Join(path, cgroupKernelMemoryLimit), []byte(strconv.FormatInt(kernelMemoryLimit, 10)), 0700); err != nil {
		// Check if the error number returned by the syscall is "EBUSY"
		// The EBUSY signal is returned on attempts to write to the
		// memory.kmem.limit_in_bytes file if the cgroup has children or
		// once tasks have been attached to the cgroup
		if pathErr, ok := err.(*os.PathError); ok {
			if errNo, ok := pathErr.Err.(syscall.Errno); ok {
				if errNo == syscall.EBUSY {
					return fmt.Errorf("failed to set %s, because either tasks have already joined this cgroup or it has children", cgroupKernelMemoryLimit)
				}
			}
		}
		return fmt.Errorf("failed to write %v to %v: %v", kernelMemoryLimit, cgroupKernelMemoryLimit, err)
	}
	return nil
}

func (s *MemoryGroup) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.Memory != 0 {
		name := "memory.limit_in_bytes"
		if s.SupportV2 {
			name = "memory.max"
		}
		if err := writeFile(path, name, strconv.FormatInt(cgroup.Resources.Memory, 10)); err != nil {
			return err
		}
	}

	if cgroup.Resources.MemoryReservation != 0 {
		name := "memory.soft_limit_in_bytes"
		if s.SupportV2 {
			name = "memory.low"
		}
		if err := writeFile(path, name, strconv.FormatInt(cgroup.Resources.MemoryReservation, 10)); err != nil {
			return err
		}
	}

	if cgroup.Resources.MemorySwap > 0 {
		name := "memory.memsw.limit_in_bytes"
		if s.SupportV2 {
			name = "memory.swap.max"
		}
		if err := writeFile(path, name, strconv.FormatInt(cgroup.Resources.MemorySwap, 10)); err != nil {
			return err
		}
	}

	if s.SupportV2 {
		return nil
	}

	if cgroup.Resources.KernelMemory > 0 {
		if err := writeFile(path, "memory.kmem.limit_in_bytes", strconv.FormatInt(cgroup.Resources.KernelMemory, 10)); err != nil {
			return err
		}
	}

	if cgroup.Resources.KernelMemoryTCP != 0 {
		if err := writeFile(path, "memory.kmem.tcp.limit_in_bytes", strconv.FormatInt(cgroup.Resources.KernelMemoryTCP, 10)); err != nil {
			return err
		}
	}
	if cgroup.Resources.OomKillDisable {
		if err := writeFile(path, "memory.oom_control", "1"); err != nil {
			return err
		}
	}
	if cgroup.Resources.MemorySwappiness == nil || int64(*cgroup.Resources.MemorySwappiness) == -1 {
		return nil
	} else if int64(*cgroup.Resources.MemorySwappiness) >= 0 && int64(*cgroup.Resources.MemorySwappiness) <= 100 {
		if err := writeFile(path, "memory.swappiness", strconv.FormatInt(*cgroup.Resources.MemorySwappiness, 10)); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("invalid value:%d. valid memory swappiness range is 0-100", int64(*cgroup.Resources.MemorySwappiness))
	}

	return nil
}

func (s *MemoryGroup) Remove(d *cgroupData) error {
	path, err := d.path("memory")
	if cgroups.IsV2Error(err) {
		err = nil
	}
	return removePath(path, err)
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

	cacheName := "cache"
	swapName := "memsw"
	rssName := "rss"
	if s.SupportV2 {
		cacheName = "file"
		swapName = "swap"
		rssName = "anon"
	}

	sc := bufio.NewScanner(statsFile)
	for sc.Scan() {
		t, v, err := getCgroupParamKeyValue(sc.Text())
		if err != nil {
			return fmt.Errorf("failed to parse memory.stat (%q) - %v", sc.Text(), err)
		}
		stats.MemoryStats.Stats[t] = v
	}

	cacheVal, ok := stats.MemoryStats.Stats[cacheName]
	if ok {
		stats.MemoryStats.Cache = cacheVal
		stats.MemoryStats.Stats["cache"] = cacheVal
	}

	rssVal, ok := stats.MemoryStats.Stats[rssName]
	if ok {
		stats.MemoryStats.Stats["rss"] = rssVal
	}

	memoryUsage, err := s.getMemoryData(path, "")
	if err != nil {
		return err
	}
	stats.MemoryStats.Usage = memoryUsage

	swapUsage, err := s.getMemoryData(path, swapName)
	if err != nil {
		return err
	}
	stats.MemoryStats.SwapUsage = swapUsage

	if s.SupportV2 {
		return nil
	}

	kernelUsage, err := s.getMemoryData(path, "kmem")
	if err != nil {
		return err
	}
	stats.MemoryStats.KernelUsage = kernelUsage

	return nil
}

func memoryAssigned(cgroup *configs.Cgroup) bool {
	return cgroup.Resources.Memory != 0 ||
		cgroup.Resources.MemoryReservation != 0 ||
		cgroup.Resources.MemorySwap > 0 ||
		cgroup.Resources.KernelMemory > 0 ||
		cgroup.Resources.KernelMemoryTCP > 0 ||
		cgroup.Resources.OomKillDisable ||
		(cgroup.Resources.MemorySwappiness != nil && *cgroup.Resources.MemorySwappiness != -1)
}

func (s *MemoryGroup) getMemoryData(path, name string) (cgroups.MemoryData, error) {
	memoryData := cgroups.MemoryData{}

	moduleName := "memory"
	if name != "" {
		moduleName = strings.Join([]string{"memory", name}, ".")
	}

	usageName := "usage_in_bytes"
	if s.SupportV2 {
		usageName = "current"
	}

	usage := strings.Join([]string{moduleName, usageName}, ".")
	value, err := getCgroupParamUint(path, usage)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s  %v", usage, err)
	}
	memoryData.Usage = value

	if s.SupportV2 {
		return memoryData, nil
	}

	maxUsage := strings.Join([]string{moduleName, "max_usage_in_bytes"}, ".")
	failcnt := strings.Join([]string{moduleName, "failcnt"}, ".")
	value, err = getCgroupParamUint(path, maxUsage)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s  %v", maxUsage, err)
	}
	memoryData.MaxUsage = value
	value, err = getCgroupParamUint(path, failcnt)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s  %v", failcnt, err)
	}
	memoryData.Failcnt = value

	limit := strings.Join([]string{moduleName, "limit_in_bytes"}, ".")

	value, err = getCgroupParamUint(path, limit)
	if err != nil {
		if moduleName != "memory" && os.IsNotExist(err) {
			return cgroups.MemoryData{}, nil
		}
		return cgroups.MemoryData{}, fmt.Errorf("failed to parse %s - %v", limit, err)
	}
	memoryData.Limit = value

	return memoryData, nil
}

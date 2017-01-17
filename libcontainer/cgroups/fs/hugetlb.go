// +build linux

package fs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

// HugetlbGroup represents hugetlb control group.
type HugetlbGroup struct {
}

// Name returns the subsystem name of the cgroup.
func (s *HugetlbGroup) Name() string {
	return "hugetlb"
}

// Apply moves the process to the cgroup, without
// setting the resource limits.
func (s *HugetlbGroup) Apply(d *cgroupData) error {
	_, err := d.join("hugetlb")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

// Set sets the reource limits to the cgroup.
func (s *HugetlbGroup) Set(path string, cgroup *configs.Cgroup) error {
	for _, hugetlb := range cgroup.Resources.HugetlbLimit {
		if err := writeFile(path, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "limit_in_bytes"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}
	}

	return nil
}

// Remove deletes the cgroup.
func (s *HugetlbGroup) Remove(d *cgroupData) error {
	return removePath(d.path("hugetlb"))
}

// GetStats returns the statistic of the cgroup.
func (s *HugetlbGroup) GetStats(path string, stats *cgroups.Stats) error {
	hugetlbStats := cgroups.HugetlbStats{}
	for _, pageSize := range HugePageSizes {
		usage := strings.Join([]string{"hugetlb", pageSize, "usage_in_bytes"}, ".")
		value, err := getCgroupParamUint(path, usage)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", usage, err)
		}
		hugetlbStats.Usage = value

		maxUsage := strings.Join([]string{"hugetlb", pageSize, "max_usage_in_bytes"}, ".")
		value, err = getCgroupParamUint(path, maxUsage)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", maxUsage, err)
		}
		hugetlbStats.MaxUsage = value

		failcnt := strings.Join([]string{"hugetlb", pageSize, "failcnt"}, ".")
		value, err = getCgroupParamUint(path, failcnt)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", failcnt, err)
		}
		hugetlbStats.Failcnt = value

		stats.HugetlbStats[pageSize] = hugetlbStats
	}

	return nil
}

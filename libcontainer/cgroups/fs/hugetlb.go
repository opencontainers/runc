// +build linux

package fs

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type HugetlbGroup struct {
}

func (s *HugetlbGroup) Name() string {
	return "hugetlb"
}

func (s *HugetlbGroup) Apply(d *cgroupData) error {
	_, err := d.join("hugetlb")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

// HasReservationAccountingSupport checks if reservation accounting of huge pages in the hugetlb cgroup
// is supported. This is supported from linux 5.7
// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/hugetlb.html
func (s *HugetlbGroup) HasReservationAccountingSupport(path string) bool {
	if len(HugePageSizes) == 0 {
		return false
	}
	_, err := fscommon.ReadFile(path, strings.Join([]string{"hugetlb", HugePageSizes[0], "rsvd", "limit_in_bytes"}, "."))
	return err == nil
}

func (s *HugetlbGroup) Set(path string, cgroup *configs.Cgroup) error {
	supportsReservationAccounting := s.HasReservationAccountingSupport(path)
	for _, hugetlb := range cgroup.Resources.HugetlbLimit {
		if err := fscommon.WriteFile(path, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "limit_in_bytes"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}

		if !supportsReservationAccounting {
			continue
		}
		if err := fscommon.WriteFile(path, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "rsvd", "limit_in_bytes"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}
	}

	return nil
}

func (s *HugetlbGroup) Remove(d *cgroupData) error {
	return removePath(d.path("hugetlb"))
}

func (s *HugetlbGroup) GetStats(path string, stats *cgroups.Stats) error {
	hugetlbStats := cgroups.HugetlbStats{}
	supportsReservationAccounting := s.HasReservationAccountingSupport(path)

	for _, pageSize := range HugePageSizes {
		filenamePrefix := strings.Join([]string{"hugetlb", pageSize}, ".")

		if supportsReservationAccounting {
			filenamePrefix += ".rsvd"
		}
		usage := fmt.Sprintf("%s.usage_in_bytes", filenamePrefix)
		value, err := fscommon.GetCgroupParamUint(path, usage)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", usage, err)
		}
		hugetlbStats.Usage = value

		maxUsage := fmt.Sprintf("%s.max_usage_in_bytes", filenamePrefix)
		value, err = fscommon.GetCgroupParamUint(path, maxUsage)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", maxUsage, err)
		}
		hugetlbStats.MaxUsage = value

		failcnt := fmt.Sprintf("%s.failcnt", filenamePrefix)
		value, err = fscommon.GetCgroupParamUint(path, failcnt)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", failcnt, err)
		}
		hugetlbStats.Failcnt = value

		stats.HugetlbStats[pageSize] = hugetlbStats
	}

	return nil
}

// +build linux

package fs2

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func setHugeTlb(dirPath string, cgroup *configs.Cgroup) error {
	for _, hugetlb := range cgroup.Resources.HugetlbLimit {
		if err := fscommon.WriteFile(dirPath, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "max"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}
	}

	return nil
}

func statHugeTlb(dirPath string, stats *cgroups.Stats, cgroup *configs.Cgroup) error {
	hugetlbStats := cgroups.HugetlbStats{}
	for _, entry := range cgroup.Resources.HugetlbLimit {
		max := strings.Join([]string{"hugetlb", entry.Pagesize, "max"}, ".")
		value, err := fscommon.GetCgroupParamUint(dirPath, max)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", max, err)
		}
		hugetlbStats.Usage = value

		usage := strings.Join([]string{"hugetlb", entry.Pagesize, "current"}, ".")
		value, err = fscommon.GetCgroupParamUint(dirPath, usage)
		if err != nil {
			return fmt.Errorf("failed to parse %s - %v", usage, err)
		}
		hugetlbStats.Usage = value

		stats.HugetlbStats[entry.Pagesize] = hugetlbStats
	}

	return nil
}

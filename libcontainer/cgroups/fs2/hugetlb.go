// +build linux

package fs2

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"

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
		usage := strings.Join([]string{"hugetlb", entry.Pagesize, "current"}, ".")
		value, err := fscommon.GetCgroupParamUint(dirPath, usage)
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.current file", entry.Pagesize)
		}
		hugetlbStats.Usage = value

		fileName := strings.Join([]string{"hugetlb", entry.Pagesize, "events"}, ".")
		filePath := filepath.Join(dirPath, fileName)
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.events file", entry.Pagesize)
		}
		_, value, err = fscommon.GetCgroupParamKeyValue(string(contents))
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.events file", entry.Pagesize)
		}
		hugetlbStats.Failcnt = value

		stats.HugetlbStats[entry.Pagesize] = hugetlbStats
	}

	return nil
}

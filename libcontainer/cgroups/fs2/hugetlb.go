// +build linux

package fs2

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func isHugeTlbSet(cgroup *configs.Cgroup) bool {
	return len(cgroup.Resources.HugetlbLimit) > 0
}

// HasReservationAccountingSupport checks if reservation accounting of huge pages in the hugetlb cgroup
// is supported. This is supported from linux 5.7
// https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v1/hugetlb.html
func HasReservationAccountingSupport(dirPath string) bool {
	hugePageSizes, err := cgroups.GetHugePageSize()
	if err != nil || len(hugePageSizes) == 0 {
		return false
	}
	_, err = fscommon.ReadFile(dirPath, strings.Join([]string{"hugetlb", hugePageSizes[0], "rsvd", "max"}, "."))
	return err == nil
}

func setHugeTlb(dirPath string, cgroup *configs.Cgroup) error {
	if !isHugeTlbSet(cgroup) {
		return nil
	}
	supportsReservationAccounting := HasReservationAccountingSupport(dirPath)
	for _, hugetlb := range cgroup.Resources.HugetlbLimit {
		if err := fscommon.WriteFile(dirPath, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "max"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}
		if !supportsReservationAccounting {
			continue
		}
		if err := fscommon.WriteFile(dirPath, strings.Join([]string{"hugetlb", hugetlb.Pagesize, "rsvd", "max"}, "."), strconv.FormatUint(hugetlb.Limit, 10)); err != nil {
			return err
		}
	}

	return nil
}

func statHugeTlb(dirPath string, stats *cgroups.Stats) error {
	hugePageSizes, err := cgroups.GetHugePageSize()
	if err != nil {
		return errors.Wrap(err, "failed to fetch hugetlb info")
	}
	hugetlbStats := cgroups.HugetlbStats{}

	supportsReservationAccounting := HasReservationAccountingSupport(dirPath)

	for _, pagesize := range hugePageSizes {
		filenamePrefix := strings.Join([]string{"hugetlb", pagesize}, ".")

		if supportsReservationAccounting {
			filenamePrefix += ".rsvd"
		}

		usage := fmt.Sprintf("%s.current", filenamePrefix)
		value, err := fscommon.GetCgroupParamUint(dirPath, usage)
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.current file", pagesize)
		}
		hugetlbStats.Usage = value

		fileName := fmt.Sprintf("%s.events", filenamePrefix)
		filePath := filepath.Join(dirPath, fileName)
		contents, err := ioutil.ReadFile(filePath)
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.events file", pagesize)
		}
		_, value, err = fscommon.GetCgroupParamKeyValue(string(contents))
		if err != nil {
			return errors.Wrapf(err, "failed to parse hugetlb.%s.events file", pagesize)
		}
		hugetlbStats.Failcnt = value

		stats.HugetlbStats[pagesize] = hugetlbStats
	}

	return nil
}

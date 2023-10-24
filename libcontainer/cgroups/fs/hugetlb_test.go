package fs

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	hugetlbUsageContents    = "128\n"
	hugetlbMaxUsageContents = "256\n"
	hugetlbFailcnt          = "100\n"
)

const (
	usage    = "hugetlb.%s.usage_in_bytes"
	limit    = "hugetlb.%s.limit_in_bytes"
	maxUsage = "hugetlb.%s.max_usage_in_bytes"
	failcnt  = "hugetlb.%s.failcnt"

	rsvdUsage    = "hugetlb.%s.rsvd.usage_in_bytes"
	rsvdLimit    = "hugetlb.%s.rsvd.limit_in_bytes"
	rsvdMaxUsage = "hugetlb.%s.rsvd.max_usage_in_bytes"
	rsvdFailcnt  = "hugetlb.%s.rsvd.failcnt"
)

func TestHugetlbSetHugetlb(t *testing.T) {
	path := tempDir(t, "hugetlb")

	const (
		hugetlbBefore = 256
		hugetlbAfter  = 512
	)

	for _, pageSize := range cgroups.HugePageSizes() {
		writeFileContents(t, path, map[string]string{
			fmt.Sprintf(limit, pageSize): strconv.Itoa(hugetlbBefore),
		})
	}

	r := &configs.Resources{}
	for _, pageSize := range cgroups.HugePageSizes() {
		r.HugetlbLimit = []*configs.HugepageLimit{
			{
				Pagesize: pageSize,
				Limit:    hugetlbAfter,
			},
		}
		hugetlb := &HugetlbGroup{}
		if err := hugetlb.Set(path, r); err != nil {
			t.Fatal(err)
		}
	}

	for _, pageSize := range cgroups.HugePageSizes() {
		for _, f := range []string{limit, rsvdLimit} {
			limit := fmt.Sprintf(f, pageSize)
			value, err := fscommon.GetCgroupParamUint(path, limit)
			if err != nil {
				t.Fatal(err)
			}
			if value != hugetlbAfter {
				t.Fatalf("Set %s failed. Expected: %v, Got: %v", limit, hugetlbAfter, value)
			}
		}
	}
}

func TestHugetlbStats(t *testing.T) {
	path := tempDir(t, "hugetlb")
	for _, pageSize := range cgroups.HugePageSizes() {
		writeFileContents(t, path, map[string]string{
			fmt.Sprintf(usage, pageSize):    hugetlbUsageContents,
			fmt.Sprintf(maxUsage, pageSize): hugetlbMaxUsageContents,
			fmt.Sprintf(failcnt, pageSize):  hugetlbFailcnt,
		})
	}

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.HugetlbStats{Usage: 128, MaxUsage: 256, Failcnt: 100}
	for _, pageSize := range cgroups.HugePageSizes() {
		expectHugetlbStatEquals(t, expectedStats, actualStats.HugetlbStats[pageSize])
	}
}

func TestHugetlbRStatsRsvd(t *testing.T) {
	path := tempDir(t, "hugetlb")
	for _, pageSize := range cgroups.HugePageSizes() {
		writeFileContents(t, path, map[string]string{
			fmt.Sprintf(rsvdUsage, pageSize):    hugetlbUsageContents,
			fmt.Sprintf(rsvdMaxUsage, pageSize): hugetlbMaxUsageContents,
			fmt.Sprintf(rsvdFailcnt, pageSize):  hugetlbFailcnt,
		})
	}

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.HugetlbStats{Usage: 128, MaxUsage: 256, Failcnt: 100}
	for _, pageSize := range cgroups.HugePageSizes() {
		expectHugetlbStatEquals(t, expectedStats, actualStats.HugetlbStats[pageSize])
	}
}

func TestHugetlbStatsNoUsageFile(t *testing.T) {
	path := tempDir(t, "hugetlb")
	writeFileContents(t, path, map[string]string{
		maxUsage: hugetlbMaxUsageContents,
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsNoMaxUsageFile(t *testing.T) {
	path := tempDir(t, "hugetlb")
	for _, pageSize := range cgroups.HugePageSizes() {
		writeFileContents(t, path, map[string]string{
			fmt.Sprintf(usage, pageSize): hugetlbUsageContents,
		})
	}

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsBadUsageFile(t *testing.T) {
	path := tempDir(t, "hugetlb")
	for _, pageSize := range cgroups.HugePageSizes() {
		writeFileContents(t, path, map[string]string{
			fmt.Sprintf(usage, pageSize): "bad",
			maxUsage:                     hugetlbMaxUsageContents,
		})
	}

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

func TestHugetlbStatsBadMaxUsageFile(t *testing.T) {
	path := tempDir(t, "hugetlb")
	writeFileContents(t, path, map[string]string{
		usage:    hugetlbUsageContents,
		maxUsage: "bad",
	})

	hugetlb := &HugetlbGroup{}
	actualStats := *cgroups.NewStats()
	err := hugetlb.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected failure")
	}
}

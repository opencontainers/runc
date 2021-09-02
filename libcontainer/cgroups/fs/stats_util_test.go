package fs

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func blkioStatEntryEquals(expected, actual []cgroups.BlkioStatEntry) error {
	if len(expected) != len(actual) {
		return errors.New("blkioStatEntries length do not match")
	}
	for i, expValue := range expected {
		actValue := actual[i]
		if expValue != actValue {
			return fmt.Errorf("expected: %v, actual: %v", expValue, actValue)
		}
	}
	return nil
}

func expectBlkioStatsEquals(t *testing.T, expected, actual cgroups.BlkioStats) {
	t.Helper()
	if err := blkioStatEntryEquals(expected.IoServiceBytesRecursive, actual.IoServiceBytesRecursive); err != nil {
		t.Errorf("blkio IoServiceBytesRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.IoServicedRecursive, actual.IoServicedRecursive); err != nil {
		t.Errorf("blkio IoServicedRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.IoQueuedRecursive, actual.IoQueuedRecursive); err != nil {
		t.Errorf("blkio IoQueuedRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.SectorsRecursive, actual.SectorsRecursive); err != nil {
		t.Errorf("blkio SectorsRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.IoServiceTimeRecursive, actual.IoServiceTimeRecursive); err != nil {
		t.Errorf("blkio IoServiceTimeRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.IoWaitTimeRecursive, actual.IoWaitTimeRecursive); err != nil {
		t.Errorf("blkio IoWaitTimeRecursive do not match: %s", err)
	}

	if err := blkioStatEntryEquals(expected.IoMergedRecursive, actual.IoMergedRecursive); err != nil {
		t.Errorf("blkio IoMergedRecursive do not match: expected: %v, actual: %v", expected.IoMergedRecursive, actual.IoMergedRecursive)
	}

	if err := blkioStatEntryEquals(expected.IoTimeRecursive, actual.IoTimeRecursive); err != nil {
		t.Errorf("blkio IoTimeRecursive do not match: %s", err)
	}
}

func expectThrottlingDataEquals(t *testing.T, expected, actual cgroups.ThrottlingData) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected throttling data: %v, actual: %v", expected, actual)
	}
}

func expectHugetlbStatEquals(t *testing.T, expected, actual cgroups.HugetlbStats) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected hugetlb stats: %v, actual: %v", expected, actual)
	}
}

func expectMemoryStatEquals(t *testing.T, expected, actual cgroups.MemoryStats) {
	t.Helper()
	expectMemoryDataEquals(t, expected.Usage, actual.Usage)
	expectMemoryDataEquals(t, expected.SwapUsage, actual.SwapUsage)
	expectMemoryDataEquals(t, expected.KernelUsage, actual.KernelUsage)
	expectPageUsageByNUMAEquals(t, expected.PageUsageByNUMA, actual.PageUsageByNUMA)

	if expected.UseHierarchy != actual.UseHierarchy {
		t.Errorf("Expected memory use hierarchy: %v, actual: %v", expected.UseHierarchy, actual.UseHierarchy)
	}

	for key, expValue := range expected.Stats {
		actValue, ok := actual.Stats[key]
		if !ok {
			t.Errorf("Expected memory stat key %s not found", key)
		}
		if expValue != actValue {
			t.Errorf("Expected memory stat value: %d, actual: %d", expValue, actValue)
		}
	}
}

func expectMemoryDataEquals(t *testing.T, expected, actual cgroups.MemoryData) {
	t.Helper()
	if expected.Usage != actual.Usage {
		t.Errorf("Expected memory usage: %d, actual: %d", expected.Usage, actual.Usage)
	}
	if expected.MaxUsage != actual.MaxUsage {
		t.Errorf("Expected memory max usage: %d, actual: %d", expected.MaxUsage, actual.MaxUsage)
	}
	if expected.Failcnt != actual.Failcnt {
		t.Errorf("Expected memory failcnt %d, actual: %d", expected.Failcnt, actual.Failcnt)
	}
	if expected.Limit != actual.Limit {
		t.Errorf("Expected memory limit: %d, actual: %d", expected.Limit, actual.Limit)
	}
}

func expectPageUsageByNUMAEquals(t *testing.T, expected, actual cgroups.PageUsageByNUMA) {
	t.Helper()
	if !reflect.DeepEqual(expected.Total, actual.Total) {
		t.Errorf("Expected total page usage by NUMA: %#v, actual: %#v", expected.Total, actual.Total)
	}
	if !reflect.DeepEqual(expected.File, actual.File) {
		t.Errorf("Expected file page usage by NUMA: %#v, actual: %#v", expected.File, actual.File)
	}
	if !reflect.DeepEqual(expected.Anon, actual.Anon) {
		t.Errorf("Expected anon page usage by NUMA: %#v, actual: %#v", expected.Anon, actual.Anon)
	}
	if !reflect.DeepEqual(expected.Unevictable, actual.Unevictable) {
		t.Errorf("Expected unevictable page usage by NUMA: %#v, actual: %#v", expected.Unevictable, actual.Unevictable)
	}
	if !reflect.DeepEqual(expected.Hierarchical.Total, actual.Hierarchical.Total) {
		t.Errorf("Expected hierarchical total page usage by NUMA: %#v, actual: %#v", expected.Hierarchical.Total, actual.Hierarchical.Total)
	}
	if !reflect.DeepEqual(expected.Hierarchical.File, actual.Hierarchical.File) {
		t.Errorf("Expected hierarchical file page usage by NUMA: %#v, actual: %#v", expected.Hierarchical.File, actual.Hierarchical.File)
	}
	if !reflect.DeepEqual(expected.Hierarchical.Anon, actual.Hierarchical.Anon) {
		t.Errorf("Expected hierarchical anon page usage by NUMA: %#v, actual: %#v", expected.Hierarchical.Anon, actual.Hierarchical.Anon)
	}
	if !reflect.DeepEqual(expected.Hierarchical.Unevictable, actual.Hierarchical.Unevictable) {
		t.Errorf("Expected hierarchical total page usage by NUMA: %#v, actual: %#v", expected.Hierarchical.Unevictable, actual.Hierarchical.Unevictable)
	}
}

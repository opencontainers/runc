package fs2

import (
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

func expectMemoryStatEquals(t *testing.T, expected, actual cgroups.MemoryStats) {
	t.Helper()
	if expected.Cache != actual.Cache {
		t.Errorf("Expected memory cache: %d, actual: %d", expected.Cache, actual.Cache)
	}
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

package fs

import (
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const (
	cpuAcctUsageContents       = "12262454190222160"
	cpuAcctUsagePerCPUContents = "1564936537989058 1583937096487821 1604195415465681 1596445226820187 1481069084155629 1478735613864327 1477610593414743 1476362015778086"
	cpuAcctStatContents        = "user 452278264\nsystem 291429664"
	cpuAcctUsageAll            = `cpu user system
	0 962250696038415 637727786389114
	1 981956408513304 638197595421064
	2 1002658817529022 638956774598358
	3 994937703492523 637985531181620
	4 874843781648690 638837766495476
	5 872544369885276 638763309884944
	6 870104915696359 640081778921247
	7 870202363887496 638716766259495
	`
)

func TestCpuacctStats(t *testing.T) {
	path := tempDir(t, "cpuacct")
	writeFileContents(t, path, map[string]string{
		"cpuacct.usage":        cpuAcctUsageContents,
		"cpuacct.usage_percpu": cpuAcctUsagePerCPUContents,
		"cpuacct.stat":         cpuAcctStatContents,
		"cpuacct.usage_all":    cpuAcctUsageAll,
	})

	cpuacct := &CpuacctGroup{}
	actualStats := *cgroups.NewStats()
	err := cpuacct.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.CpuUsage{
		TotalUsage: uint64(12262454190222160),
		PercpuUsage: []uint64{
			1564936537989058, 1583937096487821, 1604195415465681, 1596445226820187,
			1481069084155629, 1478735613864327, 1477610593414743, 1476362015778086,
		},
		PercpuUsageInKernelmode: []uint64{
			637727786389114, 638197595421064, 638956774598358, 637985531181620,
			638837766495476, 638763309884944, 640081778921247, 638716766259495,
		},
		PercpuUsageInUsermode: []uint64{
			962250696038415, 981956408513304, 1002658817529022, 994937703492523,
			874843781648690, 872544369885276, 870104915696359, 870202363887496,
		},
		UsageInKernelmode: (uint64(291429664) * nanosecondsInSecond) / clockTicks,
		UsageInUsermode:   (uint64(452278264) * nanosecondsInSecond) / clockTicks,
	}

	if !reflect.DeepEqual(expectedStats, actualStats.CpuStats.CpuUsage) {
		t.Errorf("Expected CPU usage %#v but found %#v\n",
			expectedStats, actualStats.CpuStats.CpuUsage)
	}
}

func TestCpuacctStatsWithoutUsageAll(t *testing.T) {
	path := tempDir(t, "cpuacct")
	writeFileContents(t, path, map[string]string{
		"cpuacct.usage":        cpuAcctUsageContents,
		"cpuacct.usage_percpu": cpuAcctUsagePerCPUContents,
		"cpuacct.stat":         cpuAcctStatContents,
	})

	cpuacct := &CpuacctGroup{}
	actualStats := *cgroups.NewStats()
	err := cpuacct.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.CpuUsage{
		TotalUsage: uint64(12262454190222160),
		PercpuUsage: []uint64{
			1564936537989058, 1583937096487821, 1604195415465681, 1596445226820187,
			1481069084155629, 1478735613864327, 1477610593414743, 1476362015778086,
		},
		PercpuUsageInKernelmode: []uint64{},
		PercpuUsageInUsermode:   []uint64{},
		UsageInKernelmode:       (uint64(291429664) * nanosecondsInSecond) / clockTicks,
		UsageInUsermode:         (uint64(452278264) * nanosecondsInSecond) / clockTicks,
	}

	if !reflect.DeepEqual(expectedStats, actualStats.CpuStats.CpuUsage) {
		t.Errorf("Expected CPU usage %#v but found %#v\n",
			expectedStats, actualStats.CpuStats.CpuUsage)
	}
}

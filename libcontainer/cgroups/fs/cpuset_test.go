package fs

import (
	"reflect"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	cpus                  = "0-2,7,12-14\n"
	cpuExclusive          = "1\n"
	mems                  = "1-4,6,9\n"
	memHardwall           = "0\n"
	memExclusive          = "0\n"
	memoryMigrate         = "1\n"
	memorySpreadPage      = "0\n"
	memorySpeadSlab       = "1\n"
	memoryPressure        = "34377\n"
	schedLoadBalance      = "1\n"
	schedRelaxDomainLevel = "-1\n"
)

var cpusetTestFiles = map[string]string{
	"cpuset.cpus":                     cpus,
	"cpuset.cpu_exclusive":            cpuExclusive,
	"cpuset.mems":                     mems,
	"cpuset.mem_hardwall":             memHardwall,
	"cpuset.mem_exclusive":            memExclusive,
	"cpuset.memory_migrate":           memoryMigrate,
	"cpuset.memory_spread_page":       memorySpreadPage,
	"cpuset.memory_spread_slab":       memorySpeadSlab,
	"cpuset.memory_pressure":          memoryPressure,
	"cpuset.sched_load_balance":       schedLoadBalance,
	"cpuset.sched_relax_domain_level": schedRelaxDomainLevel,
}

func TestCPUSetSetCpus(t *testing.T) {
	path := tempDir(t, "cpuset")

	const (
		cpusBefore = "0"
		cpusAfter  = "1-3"
	)

	writeFileContents(t, path, map[string]string{
		"cpuset.cpus": cpusBefore,
	})

	r := &configs.Resources{
		CpusetCpus: cpusAfter,
	}
	cpuset := &CpusetGroup{}
	if err := cpuset.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "cpuset.cpus")
	if err != nil {
		t.Fatal(err)
	}
	if value != cpusAfter {
		t.Fatal("Got the wrong value, set cpuset.cpus failed.")
	}
}

func TestCPUSetSetMems(t *testing.T) {
	path := tempDir(t, "cpuset")

	const (
		memsBefore = "0"
		memsAfter  = "1"
	)

	writeFileContents(t, path, map[string]string{
		"cpuset.mems": memsBefore,
	})

	r := &configs.Resources{
		CpusetMems: memsAfter,
	}
	cpuset := &CpusetGroup{}
	if err := cpuset.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "cpuset.mems")
	if err != nil {
		t.Fatal(err)
	}
	if value != memsAfter {
		t.Fatal("Got the wrong value, set cpuset.mems failed.")
	}
}

func TestCPUSetStatsCorrect(t *testing.T) {
	path := tempDir(t, "cpuset")
	writeFileContents(t, path, cpusetTestFiles)

	cpuset := &CpusetGroup{}
	actualStats := *cgroups.NewStats()
	err := cpuset.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}
	expectedStats := cgroups.CPUSetStats{
		CPUs:                  []uint16{0, 1, 2, 7, 12, 13, 14},
		CPUExclusive:          1,
		Mems:                  []uint16{1, 2, 3, 4, 6, 9},
		MemoryMigrate:         1,
		MemHardwall:           0,
		MemExclusive:          0,
		MemorySpreadPage:      0,
		MemorySpreadSlab:      1,
		MemoryPressure:        34377,
		SchedLoadBalance:      1,
		SchedRelaxDomainLevel: -1,
	}
	if !reflect.DeepEqual(expectedStats, actualStats.CPUSetStats) {
		t.Fatalf("Expected Cpuset stats usage %#v but found %#v",
			expectedStats, actualStats.CPUSetStats)
	}
}

func TestCPUSetStatsMissingFiles(t *testing.T) {
	for _, testCase := range []struct {
		desc               string
		filename, contents string
		removeFile         bool
	}{
		{
			desc:       "empty cpus file",
			filename:   "cpuset.cpus",
			contents:   "",
			removeFile: false,
		},
		{
			desc:       "empty mems file",
			filename:   "cpuset.mems",
			contents:   "",
			removeFile: false,
		},
		{
			desc:       "corrupted cpus file",
			filename:   "cpuset.cpus",
			contents:   "0-3,*4^2",
			removeFile: false,
		},
		{
			desc:       "corrupted mems file",
			filename:   "cpuset.mems",
			contents:   "0,1,2-5,8-7",
			removeFile: false,
		},
		{
			desc:       "missing cpu_exclusive file",
			filename:   "cpuset.cpu_exclusive",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing memory_migrate file",
			filename:   "cpuset.memory_migrate",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing mem_hardwall file",
			filename:   "cpuset.mem_hardwall",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing mem_exclusive file",
			filename:   "cpuset.mem_exclusive",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing memory_spread_page file",
			filename:   "cpuset.memory_spread_page",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing memory_spread_slab file",
			filename:   "cpuset.memory_spread_slab",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing memory_pressure file",
			filename:   "cpuset.memory_pressure",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing sched_load_balance file",
			filename:   "cpuset.sched_load_balance",
			contents:   "",
			removeFile: true,
		},
		{
			desc:       "missing sched_relax_domain_level file",
			filename:   "cpuset.sched_relax_domain_level",
			contents:   "",
			removeFile: true,
		},
	} {
		t.Run(testCase.desc, func(t *testing.T) {
			path := tempDir(t, "cpuset")

			tempCpusetTestFiles := map[string]string{}
			for i, v := range cpusetTestFiles {
				tempCpusetTestFiles[i] = v
			}

			if testCase.removeFile {
				delete(tempCpusetTestFiles, testCase.filename)
				writeFileContents(t, path, tempCpusetTestFiles)
				cpuset := &CpusetGroup{}
				actualStats := *cgroups.NewStats()
				err := cpuset.GetStats(path, &actualStats)
				if err != nil {
					t.Errorf("failed unexpectedly: %q", err)
				}
			} else {
				tempCpusetTestFiles[testCase.filename] = testCase.contents
				writeFileContents(t, path, tempCpusetTestFiles)
				cpuset := &CpusetGroup{}
				actualStats := *cgroups.NewStats()
				err := cpuset.GetStats(path, &actualStats)

				if err == nil {
					t.Error("failed to return expected error")
				}
			}
		})
	}
}

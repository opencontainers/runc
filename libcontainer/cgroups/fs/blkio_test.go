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
	sectorsRecursiveContents      = `8:0 1024`
	sectorsRecursiveContentsBFQ   = `8:0 2048`
	serviceBytesRecursiveContents = `8:0 Read 100
8:0 Write 200
8:0 Sync 300
8:0 Async 500
8:0 Total 500
Total 500`

	serviceBytesRecursiveContentsBFQ = `8:0 Read 1100
8:0 Write 1200
8:0 Sync 1300
8:0 Async 1500
8:0 Total 1500
Total 1500`
	servicedRecursiveContents = `8:0 Read 10
8:0 Write 40
8:0 Sync 20
8:0 Async 30
8:0 Total 50
Total 50`
	servicedRecursiveContentsBFQ = `8:0 Read 11
8:0 Write 41
8:0 Sync 21
8:0 Async 31
8:0 Total 51
Total 51`
	queuedRecursiveContents = `8:0 Read 1
8:0 Write 4
8:0 Sync 2
8:0 Async 3
8:0 Total 5
Total 5`
	queuedRecursiveContentsBFQ = `8:0 Read 2
8:0 Write 3
8:0 Sync 4
8:0 Async 5
8:0 Total 6
Total 6`
	serviceTimeRecursiveContents = `8:0 Read 173959
8:0 Write 0
8:0 Sync 0
8:0 Async 173959
8:0 Total 17395
Total 17395`
	serviceTimeRecursiveContentsBFQ = `8:0 Read 173959
8:0 Write 0
8:0 Sync 0
8:0 Async 173
8:0 Total 174
Total 174`
	waitTimeRecursiveContents = `8:0 Read 15571
8:0 Write 0
8:0 Sync 0
8:0 Async 15571
8:0 Total 15571`
	waitTimeRecursiveContentsBFQ = `8:0 Read 1557
8:0 Write 0
8:0 Sync 0
8:0 Async 1557
8:0 Total 1557`
	mergedRecursiveContents = `8:0 Read 5
8:0 Write 10
8:0 Sync 0
8:0 Async 0
8:0 Total 15
Total 15`
	mergedRecursiveContentsBFQ = `8:0 Read 51
8:0 Write 101
8:0 Sync 0
8:0 Async 0
8:0 Total 151
Total 151`
	timeRecursiveContents    = `8:0 8`
	timeRecursiveContentsBFQ = `8:0 16`
	throttleServiceBytes     = `8:0 Read 11030528
8:0 Write 23
8:0 Sync 42
8:0 Async 11030528
8:0 Total 11030528
252:0 Read 11030528
252:0 Write 23
252:0 Sync 42
252:0 Async 11030528
252:0 Total 11030528
Total 22061056`
	throttleServiceBytesRecursive = `8:0 Read 110305281
8:0 Write 231
8:0 Sync 421
8:0 Async 110305281
8:0 Total 110305281
252:0 Read 110305281
252:0 Write 231
252:0 Sync 421
252:0 Async 110305281
252:0 Total 110305281
Total 220610561`
	throttleServiced = `8:0 Read 164
8:0 Write 23
8:0 Sync 42
8:0 Async 164
8:0 Total 164
252:0 Read 164
252:0 Write 23
252:0 Sync 42
252:0 Async 164
252:0 Total 164
Total 328`
	throttleServicedRecursive = `8:0 Read 1641
8:0 Write 231
8:0 Sync 421
8:0 Async 1641
8:0 Total 1641
252:0 Read 1641
252:0 Write 231
252:0 Sync 421
252:0 Async 1641
252:0 Total 1641
Total 3281`
)

var blkioBFQDebugStatsTestFiles = map[string]string{
	"blkio.bfq.io_service_bytes_recursive": serviceBytesRecursiveContentsBFQ,
	"blkio.bfq.io_serviced_recursive":      servicedRecursiveContentsBFQ,
	"blkio.bfq.io_queued_recursive":        queuedRecursiveContentsBFQ,
	"blkio.bfq.io_service_time_recursive":  serviceTimeRecursiveContentsBFQ,
	"blkio.bfq.io_wait_time_recursive":     waitTimeRecursiveContentsBFQ,
	"blkio.bfq.io_merged_recursive":        mergedRecursiveContentsBFQ,
	"blkio.bfq.time_recursive":             timeRecursiveContentsBFQ,
	"blkio.bfq.sectors_recursive":          sectorsRecursiveContentsBFQ,
}

var blkioBFQStatsTestFiles = map[string]string{
	"blkio.bfq.io_service_bytes_recursive": serviceBytesRecursiveContentsBFQ,
	"blkio.bfq.io_serviced_recursive":      servicedRecursiveContentsBFQ,
}

var blkioCFQStatsTestFiles = map[string]string{
	"blkio.io_service_bytes_recursive": serviceBytesRecursiveContents,
	"blkio.io_serviced_recursive":      servicedRecursiveContents,
	"blkio.io_queued_recursive":        queuedRecursiveContents,
	"blkio.io_service_time_recursive":  serviceTimeRecursiveContents,
	"blkio.io_wait_time_recursive":     waitTimeRecursiveContents,
	"blkio.io_merged_recursive":        mergedRecursiveContents,
	"blkio.time_recursive":             timeRecursiveContents,
	"blkio.sectors_recursive":          sectorsRecursiveContents,
}

type blkioStatFailureTestCase struct {
	desc     string
	filename string
}

func appendBlkioStatEntry(blkioStatEntries *[]cgroups.BlkioStatEntry, major, minor, value uint64, op string) { //nolint:unparam
	*blkioStatEntries = append(*blkioStatEntries, cgroups.BlkioStatEntry{Major: major, Minor: minor, Value: value, Op: op})
}

func TestBlkioSetWeight(t *testing.T) {
	const (
		weightBefore = 100
		weightAfter  = 200
	)

	for _, legacyIOScheduler := range []bool{false, true} {
		// Populate cgroup
		path := tempDir(t, "blkio")
		weightFilename := "blkio.bfq.weight"
		if legacyIOScheduler {
			weightFilename = "blkio.weight"
		}
		writeFileContents(t, path, map[string]string{
			weightFilename: strconv.Itoa(weightBefore),
		})
		// Apply new configuration
		r := &configs.Resources{
			BlkioWeight: weightAfter,
		}
		blkio := &BlkioGroup{}
		if err := blkio.Set(path, r); err != nil {
			t.Fatal(err)
		}
		// Verify results
		if weightFilename != blkio.weightFilename {
			t.Fatalf("weight filename detection failed: expected %q, detected %q", weightFilename, blkio.weightFilename)
		}
		value, err := fscommon.GetCgroupParamUint(path, weightFilename)
		if err != nil {
			t.Fatal(err)
		}
		if value != weightAfter {
			t.Fatalf("Got the wrong value, set %s failed.", weightFilename)
		}
	}
}

func TestBlkioSetWeightDevice(t *testing.T) {
	const (
		weightDeviceBefore = "8:0 400"
	)

	for _, legacyIOScheduler := range []bool{false, true} {
		// Populate cgroup
		path := tempDir(t, "blkio")
		weightFilename := "blkio.bfq.weight"
		weightDeviceFilename := "blkio.bfq.weight_device"
		if legacyIOScheduler {
			weightFilename = "blkio.weight"
			weightDeviceFilename = "blkio.weight_device"
		}
		writeFileContents(t, path, map[string]string{
			weightFilename:       "",
			weightDeviceFilename: weightDeviceBefore,
		})
		// Apply new configuration
		wd := configs.NewWeightDevice(8, 0, 500, 0)
		weightDeviceAfter := wd.WeightString()
		r := &configs.Resources{
			BlkioWeightDevice: []*configs.WeightDevice{wd},
		}
		blkio := &BlkioGroup{}
		if err := blkio.Set(path, r); err != nil {
			t.Fatal(err)
		}
		// Verify results
		if weightDeviceFilename != blkio.weightDeviceFilename {
			t.Fatalf("weight_device filename detection failed: expected %q, detected %q", weightDeviceFilename, blkio.weightDeviceFilename)
		}
		value, err := fscommon.GetCgroupParamString(path, weightDeviceFilename)
		if err != nil {
			t.Fatal(err)
		}
		if value != weightDeviceAfter {
			t.Fatalf("Got the wrong value, set %s failed.", weightDeviceFilename)
		}
	}
}

// regression #274
func TestBlkioSetMultipleWeightDevice(t *testing.T) {
	path := tempDir(t, "blkio")

	const (
		weightDeviceBefore = "8:0 400"
	)

	wd1 := configs.NewWeightDevice(8, 0, 500, 0)
	wd2 := configs.NewWeightDevice(8, 16, 500, 0)
	// we cannot actually set and check both because normal os.WriteFile
	// when writing to cgroup file will overwrite the whole file content instead
	// of updating it as the kernel is doing. Just check the second device
	// is present will suffice for the test to ensure multiple writes are done.
	weightDeviceAfter := wd2.WeightString()

	blkio := &BlkioGroup{}
	blkio.detectWeightFilenames(path)
	if blkio.weightDeviceFilename != "blkio.bfq.weight_device" {
		t.Fatalf("when blkio controller is unavailable, expected to use \"blkio.bfq.weight_device\", tried to use %q", blkio.weightDeviceFilename)
	}
	writeFileContents(t, path, map[string]string{
		blkio.weightDeviceFilename: weightDeviceBefore,
	})

	r := &configs.Resources{
		BlkioWeightDevice: []*configs.WeightDevice{wd1, wd2},
	}
	if err := blkio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, blkio.weightDeviceFilename)
	if err != nil {
		t.Fatal(err)
	}
	if value != weightDeviceAfter {
		t.Fatalf("Got the wrong value, set %s failed.", blkio.weightDeviceFilename)
	}
}

func TestBlkioBFQDebugStats(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, blkioBFQDebugStatsTestFiles)
	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.BlkioStats{}
	appendBlkioStatEntry(&expectedStats.SectorsRecursive, 8, 0, 2048, "")

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1100, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1200, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1300, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 11, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 41, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 21, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 31, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 51, "Total")

	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 2, "Read")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 3, "Write")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 4, "Sync")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 5, "Async")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 6, "Total")

	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173959, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 174, "Total")

	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Read")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Async")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Total")

	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 51, "Read")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 101, "Write")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Async")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 151, "Total")

	appendBlkioStatEntry(&expectedStats.IoTimeRecursive, 8, 0, 16, "")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestBlkioMultipleStatsFiles(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, blkioBFQDebugStatsTestFiles)
	writeFileContents(t, path, blkioCFQStatsTestFiles)
	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.BlkioStats{}
	appendBlkioStatEntry(&expectedStats.SectorsRecursive, 8, 0, 2048, "")

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1100, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1200, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1300, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 11, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 41, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 21, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 31, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 51, "Total")

	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 2, "Read")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 3, "Write")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 4, "Sync")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 5, "Async")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 6, "Total")

	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173959, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 174, "Total")

	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Read")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Async")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 1557, "Total")

	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 51, "Read")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 101, "Write")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Async")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 151, "Total")

	appendBlkioStatEntry(&expectedStats.IoTimeRecursive, 8, 0, 16, "")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestBlkioBFQStats(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, blkioBFQStatsTestFiles)
	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	expectedStats := cgroups.BlkioStats{}

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1100, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1200, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1300, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 1500, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 11, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 41, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 21, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 31, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 51, "Total")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestBlkioStatsNoFilesBFQDebug(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	testCases := []blkioStatFailureTestCase{
		{
			desc:     "missing blkio.bfq.io_service_bytes_recursive file",
			filename: "blkio.bfq.io_service_bytes_recursive",
		},
		{
			desc:     "missing blkio.bfq.io_serviced_recursive file",
			filename: "blkio.bfq.io_serviced_recursive",
		},
		{
			desc:     "missing blkio.bfq.io_queued_recursive file",
			filename: "blkio.bfq.io_queued_recursive",
		},
		{
			desc:     "missing blkio.bfq.sectors_recursive file",
			filename: "blkio.bfq.sectors_recursive",
		},
		{
			desc:     "missing blkio.bfq.io_service_time_recursive file",
			filename: "blkio.bfq.io_service_time_recursive",
		},
		{
			desc:     "missing blkio.bfq.io_wait_time_recursive file",
			filename: "blkio.bfq.io_wait_time_recursive",
		},
		{
			desc:     "missing blkio.bfq.io_merged_recursive file",
			filename: "blkio.bfq.io_merged_recursive",
		},
		{
			desc:     "missing blkio.bfq.time_recursive file",
			filename: "blkio.bfq.time_recursive",
		},
	}

	for _, testCase := range testCases {
		path := tempDir(t, "cpuset")

		tempBlkioTestFiles := map[string]string{}
		for i, v := range blkioBFQDebugStatsTestFiles {
			tempBlkioTestFiles[i] = v
		}
		delete(tempBlkioTestFiles, testCase.filename)

		writeFileContents(t, path, tempBlkioTestFiles)
		cpuset := &CpusetGroup{}
		actualStats := *cgroups.NewStats()
		err := cpuset.GetStats(path, &actualStats)
		if err != nil {
			t.Errorf(fmt.Sprintf("test case '%s' failed unexpectedly: %s", testCase.desc, err))
		}
	}
}

func TestBlkioCFQStats(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, blkioCFQStatsTestFiles)

	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	// Verify expected stats.
	expectedStats := cgroups.BlkioStats{}
	appendBlkioStatEntry(&expectedStats.SectorsRecursive, 8, 0, 1024, "")

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 100, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 200, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 300, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 500, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 500, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 10, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 40, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 20, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 30, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 50, "Total")

	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 1, "Read")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 4, "Write")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 2, "Sync")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 3, "Async")
	appendBlkioStatEntry(&expectedStats.IoQueuedRecursive, 8, 0, 5, "Total")

	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173959, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 173959, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceTimeRecursive, 8, 0, 17395, "Total")

	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 15571, "Read")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Write")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 15571, "Async")
	appendBlkioStatEntry(&expectedStats.IoWaitTimeRecursive, 8, 0, 15571, "Total")

	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 5, "Read")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 10, "Write")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Sync")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 0, "Async")
	appendBlkioStatEntry(&expectedStats.IoMergedRecursive, 8, 0, 15, "Total")

	appendBlkioStatEntry(&expectedStats.IoTimeRecursive, 8, 0, 8, "")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestBlkioStatsNoFilesCFQ(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	testCases := []blkioStatFailureTestCase{
		{
			desc:     "missing blkio.io_service_bytes_recursive file",
			filename: "blkio.io_service_bytes_recursive",
		},
		{
			desc:     "missing blkio.io_serviced_recursive file",
			filename: "blkio.io_serviced_recursive",
		},
		{
			desc:     "missing blkio.io_queued_recursive file",
			filename: "blkio.io_queued_recursive",
		},
		{
			desc:     "missing blkio.sectors_recursive file",
			filename: "blkio.sectors_recursive",
		},
		{
			desc:     "missing blkio.io_service_time_recursive file",
			filename: "blkio.io_service_time_recursive",
		},
		{
			desc:     "missing blkio.io_wait_time_recursive file",
			filename: "blkio.io_wait_time_recursive",
		},
		{
			desc:     "missing blkio.io_merged_recursive file",
			filename: "blkio.io_merged_recursive",
		},
		{
			desc:     "missing blkio.time_recursive file",
			filename: "blkio.time_recursive",
		},
	}

	for _, testCase := range testCases {
		path := tempDir(t, "cpuset")

		tempBlkioTestFiles := map[string]string{}
		for i, v := range blkioCFQStatsTestFiles {
			tempBlkioTestFiles[i] = v
		}
		delete(tempBlkioTestFiles, testCase.filename)

		writeFileContents(t, path, tempBlkioTestFiles)
		cpuset := &CpusetGroup{}
		actualStats := *cgroups.NewStats()
		err := cpuset.GetStats(path, &actualStats)
		if err != nil {
			t.Errorf(fmt.Sprintf("test case '%s' failed unexpectedly: %s", testCase.desc, err))
		}
	}
}

func TestBlkioStatsUnexpectedNumberOfFields(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, map[string]string{
		"blkio.io_service_bytes_recursive": "8:0 Read 100 100",
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
		"blkio.io_service_time_recursive":  serviceTimeRecursiveContents,
		"blkio.io_wait_time_recursive":     waitTimeRecursiveContents,
		"blkio.io_merged_recursive":        mergedRecursiveContents,
		"blkio.time_recursive":             timeRecursiveContents,
	})

	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestBlkioStatsUnexpectedFieldType(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, map[string]string{
		"blkio.io_service_bytes_recursive": "8:0 Read Write",
		"blkio.io_serviced_recursive":      servicedRecursiveContents,
		"blkio.io_queued_recursive":        queuedRecursiveContents,
		"blkio.sectors_recursive":          sectorsRecursiveContents,
		"blkio.io_service_time_recursive":  serviceTimeRecursiveContents,
		"blkio.io_wait_time_recursive":     waitTimeRecursiveContents,
		"blkio.io_merged_recursive":        mergedRecursiveContents,
		"blkio.time_recursive":             timeRecursiveContents,
	})

	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err == nil {
		t.Fatal("Expected to fail, but did not")
	}
}

func TestThrottleRecursiveBlkioStats(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, map[string]string{
		"blkio.io_service_bytes_recursive":          "",
		"blkio.io_serviced_recursive":               "",
		"blkio.io_queued_recursive":                 "",
		"blkio.sectors_recursive":                   "",
		"blkio.io_service_time_recursive":           "",
		"blkio.io_wait_time_recursive":              "",
		"blkio.io_merged_recursive":                 "",
		"blkio.time_recursive":                      "",
		"blkio.throttle.io_service_bytes_recursive": throttleServiceBytesRecursive,
		"blkio.throttle.io_serviced_recursive":      throttleServicedRecursive,
	})

	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	// Verify expected stats.
	expectedStats := cgroups.BlkioStats{}

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 110305281, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 231, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 421, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 110305281, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 110305281, "Total")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 110305281, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 231, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 421, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 110305281, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 110305281, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 1641, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 231, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 421, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 1641, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 1641, "Total")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 1641, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 231, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 421, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 1641, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 1641, "Total")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestThrottleBlkioStats(t *testing.T) {
	path := tempDir(t, "blkio")
	writeFileContents(t, path, map[string]string{
		"blkio.io_service_bytes_recursive": "",
		"blkio.io_serviced_recursive":      "",
		"blkio.io_queued_recursive":        "",
		"blkio.sectors_recursive":          "",
		"blkio.io_service_time_recursive":  "",
		"blkio.io_wait_time_recursive":     "",
		"blkio.io_merged_recursive":        "",
		"blkio.time_recursive":             "",
		"blkio.throttle.io_service_bytes":  throttleServiceBytes,
		"blkio.throttle.io_serviced":       throttleServiced,
	})

	blkio := &BlkioGroup{}
	actualStats := *cgroups.NewStats()
	err := blkio.GetStats(path, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	// Verify expected stats.
	expectedStats := cgroups.BlkioStats{}

	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 11030528, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 23, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 42, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 11030528, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 8, 0, 11030528, "Total")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 11030528, "Read")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 23, "Write")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 42, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 11030528, "Async")
	appendBlkioStatEntry(&expectedStats.IoServiceBytesRecursive, 252, 0, 11030528, "Total")

	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 164, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 23, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 42, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 164, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 8, 0, 164, "Total")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 164, "Read")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 23, "Write")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 42, "Sync")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 164, "Async")
	appendBlkioStatEntry(&expectedStats.IoServicedRecursive, 252, 0, 164, "Total")

	expectBlkioStatsEquals(t, expectedStats, actualStats.BlkioStats)
}

func TestBlkioSetThrottleReadBpsDevice(t *testing.T) {
	path := tempDir(t, "blkio")

	const (
		throttleBefore = `8:0 1024`
	)

	td := configs.NewThrottleDevice(8, 0, 2048)
	throttleAfter := td.String()

	writeFileContents(t, path, map[string]string{
		"blkio.throttle.read_bps_device": throttleBefore,
	})

	r := &configs.Resources{
		BlkioThrottleReadBpsDevice: []*configs.ThrottleDevice{td},
	}
	blkio := &BlkioGroup{}
	if err := blkio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "blkio.throttle.read_bps_device")
	if err != nil {
		t.Fatal(err)
	}
	if value != throttleAfter {
		t.Fatal("Got the wrong value, set blkio.throttle.read_bps_device failed.")
	}
}

func TestBlkioSetThrottleWriteBpsDevice(t *testing.T) {
	path := tempDir(t, "blkio")

	const (
		throttleBefore = `8:0 1024`
	)

	td := configs.NewThrottleDevice(8, 0, 2048)
	throttleAfter := td.String()

	writeFileContents(t, path, map[string]string{
		"blkio.throttle.write_bps_device": throttleBefore,
	})

	r := &configs.Resources{
		BlkioThrottleWriteBpsDevice: []*configs.ThrottleDevice{td},
	}
	blkio := &BlkioGroup{}
	if err := blkio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "blkio.throttle.write_bps_device")
	if err != nil {
		t.Fatal(err)
	}
	if value != throttleAfter {
		t.Fatal("Got the wrong value, set blkio.throttle.write_bps_device failed.")
	}
}

func TestBlkioSetThrottleReadIOpsDevice(t *testing.T) {
	path := tempDir(t, "blkio")

	const (
		throttleBefore = `8:0 1024`
	)

	td := configs.NewThrottleDevice(8, 0, 2048)
	throttleAfter := td.String()

	writeFileContents(t, path, map[string]string{
		"blkio.throttle.read_iops_device": throttleBefore,
	})

	r := &configs.Resources{
		BlkioThrottleReadIOPSDevice: []*configs.ThrottleDevice{td},
	}
	blkio := &BlkioGroup{}
	if err := blkio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "blkio.throttle.read_iops_device")
	if err != nil {
		t.Fatal(err)
	}
	if value != throttleAfter {
		t.Fatal("Got the wrong value, set blkio.throttle.read_iops_device failed.")
	}
}

func TestBlkioSetThrottleWriteIOpsDevice(t *testing.T) {
	path := tempDir(t, "blkio")

	const (
		throttleBefore = `8:0 1024`
	)

	td := configs.NewThrottleDevice(8, 0, 2048)
	throttleAfter := td.String()

	writeFileContents(t, path, map[string]string{
		"blkio.throttle.write_iops_device": throttleBefore,
	})

	r := &configs.Resources{
		BlkioThrottleWriteIOPSDevice: []*configs.ThrottleDevice{td},
	}
	blkio := &BlkioGroup{}
	if err := blkio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "blkio.throttle.write_iops_device")
	if err != nil {
		t.Fatal(err)
	}
	if value != throttleAfter {
		t.Fatal("Got the wrong value, set blkio.throttle.write_iops_device failed.")
	}
}

package fs2

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const exampleIoStatData = `254:1 rbytes=6901432320 wbytes=14245535744 rios=263278 wios=248603 dbytes=0 dios=0
254:0 rbytes=2702336 wbytes=0 rios=97 wios=0 dbytes=0 dios=0
259:0 rbytes=6911345664 wbytes=14245536256 rios=264538 wios=244914 dbytes=530485248 dios=2`

var exampleIoStatsParsed = cgroups.BlkioStats{
	IoServiceBytesRecursive: []cgroups.BlkioStatEntry{
		{Major: 254, Minor: 1, Value: 6901432320, Op: "Read"},
		{Major: 254, Minor: 1, Value: 14245535744, Op: "Write"},
		{Major: 254, Minor: 0, Value: 2702336, Op: "Read"},
		{Major: 254, Minor: 0, Value: 0, Op: "Write"},
		{Major: 259, Minor: 0, Value: 6911345664, Op: "Read"},
		{Major: 259, Minor: 0, Value: 14245536256, Op: "Write"},
	},
	IoServicedRecursive: []cgroups.BlkioStatEntry{
		{Major: 254, Minor: 1, Value: 263278, Op: "Read"},
		{Major: 254, Minor: 1, Value: 248603, Op: "Write"},
		{Major: 254, Minor: 0, Value: 97, Op: "Read"},
		{Major: 254, Minor: 0, Value: 0, Op: "Write"},
		{Major: 259, Minor: 0, Value: 264538, Op: "Read"},
		{Major: 259, Minor: 0, Value: 244914, Op: "Write"},
	},
}

func lessBlkioStatEntry(a, b cgroups.BlkioStatEntry) bool {
	if a.Major != b.Major {
		return a.Major < b.Major
	}
	if a.Minor != b.Minor {
		return a.Minor < b.Minor
	}
	if a.Op != b.Op {
		return a.Op < b.Op
	}
	return a.Value < b.Value
}

func sortBlkioStats(stats *cgroups.BlkioStats) {
	for _, table := range []*[]cgroups.BlkioStatEntry{
		&stats.IoServicedRecursive,
		&stats.IoServiceBytesRecursive,
	} {
		sort.SliceStable(*table, func(i, j int) bool { return lessBlkioStatEntry((*table)[i], (*table)[j]) })
	}
}

func TestStatIo(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true

	fakeCgroupDir := t.TempDir()
	statPath := filepath.Join(fakeCgroupDir, "io.stat")

	if err := os.WriteFile(statPath, []byte(exampleIoStatData), 0o644); err != nil {
		t.Fatal(err)
	}

	var gotStats cgroups.Stats
	if err := statIo(fakeCgroupDir, &gotStats); err != nil {
		t.Error(err)
	}

	// Sort the output since statIo uses a map internally.
	sortBlkioStats(&gotStats.BlkioStats)
	sortBlkioStats(&exampleIoStatsParsed)

	if !reflect.DeepEqual(gotStats.BlkioStats, exampleIoStatsParsed) {
		t.Errorf("parsed cgroupv2 io.stat doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.BlkioStats, exampleIoStatsParsed)
	}
}

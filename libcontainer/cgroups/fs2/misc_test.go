package fs2

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
)

const exampleMiscCurrentData = `res_a 123
res_b 456
res_c 42`

const exampleMiscEventsData = `res_a.max 1
res_b.max 2
res_c.max 3`

func TestStatMiscPodCgroupEmpty(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true
	fakeCgroupDir := t.TempDir()

	// create empty misc.current and misc.events files to test the common case
	// where no misc resource keys are available
	for _, file := range []string{"misc.current", "misc.events"} {
		if _, err := os.Create(filepath.Join(fakeCgroupDir, file)); err != nil {
			t.Fatal(err)
		}
	}

	gotStats := cgroups.NewStats()

	err := statMisc(fakeCgroupDir, gotStats)
	if err != nil {
		t.Errorf("expected no error when statting empty misc.current/misc.events for cgroupv2, but got %#v", err)
	}

	if len(gotStats.MiscStats) != 0 {
		t.Errorf("parsed cgroupv2 misc.* returns unexpected resources: got %#v but expected nothing", gotStats.MiscStats)
	}
}

func TestStatMiscPodCgroupNotFound(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true
	fakeCgroupDir := t.TempDir()

	// only write misc.current to ensure pod cgroup usage
	// still reads misc.events.
	statPath := filepath.Join(fakeCgroupDir, "misc.current")
	if err := os.WriteFile(statPath, []byte(exampleMiscCurrentData), 0o644); err != nil {
		t.Fatal(err)
	}

	gotStats := cgroups.NewStats()

	// use a fake root path to mismatch the file we wrote.
	// this triggers the non-root path which should fail to find misc.events.
	err := statMisc(fakeCgroupDir, gotStats)
	if err == nil {
		t.Errorf("expected error when statting misc.current for cgroupv2 root, but was nil")
	}

	if !strings.Contains(err.Error(), "misc.events: no such file or directory") {
		t.Errorf("expected error to contain 'misc.events: no such file or directory', but was %s", err.Error())
	}
}

func TestStatMiscPodCgroup(t *testing.T) {
	// We're using a fake cgroupfs.
	cgroups.TestMode = true
	fakeCgroupDir := t.TempDir()

	currentPath := filepath.Join(fakeCgroupDir, "misc.current")
	if err := os.WriteFile(currentPath, []byte(exampleMiscCurrentData), 0o644); err != nil {
		t.Fatal(err)
	}

	eventsPath := filepath.Join(fakeCgroupDir, "misc.events")
	if err := os.WriteFile(eventsPath, []byte(exampleMiscEventsData), 0o644); err != nil {
		t.Fatal(err)
	}

	gotStats := cgroups.NewStats()

	// use a fake root path to trigger the pod cgroup lookup.
	err := statMisc(fakeCgroupDir, gotStats)
	if err != nil {
		t.Errorf("expected no error when statting misc for cgroupv2 root, but got %#+v", err)
	}

	// make sure all res_* from exampleMisc*Data are returned
	if len(gotStats.MiscStats) != 3 {
		t.Errorf("parsed cgroupv2 misc doesn't return all expected resources: \ngot %#v\nexpected %#v\n", len(gotStats.MiscStats), 3)
	}

	var expectedUsageBytes uint64 = 42
	if gotStats.MiscStats["res_c"].Usage != expectedUsageBytes {
		t.Errorf("parsed cgroupv2 misc.current for res_c doesn't match expected result: \ngot %#v\nexpected %#v\n", gotStats.MiscStats["res_c"].Usage, expectedUsageBytes)
	}
}

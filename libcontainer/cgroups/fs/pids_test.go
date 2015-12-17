// +build linux

package fs

import (
	"os"
	"strconv"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestSetPids(t *testing.T) {
	helper := NewCgroupTestUtil("pids", t)
	defer helper.cleanup()

	pidsArray := make([]uint32, 1)
	pidsArray[0] = uint32(os.Getpid())

	helper.CgroupData.c.Pids = pidsArray
	pids := &PidsGroup{}
	if err := pids.Set(helper.CgroupPath, helper.CgroupData.c); err != nil {
		t.Fatal(err)
	}
}

func TestGetPidsStats(t *testing.T) {
	helper := NewCgroupTestUtil("pids", t)
	defer helper.cleanup()

	helper.writeFileContents(map[string]string{
		"pids.current": strconv.Itoa(os.Getpid()),
		"pids.max":     "max",
	})

	actualStats := *cgroups.NewStats()
	pids := &PidsGroup{}
	err := pids.GetStats(helper.CgroupPath, &actualStats)
	if err != nil {
		t.Fatal(err)
	}

	if actualStats.PidsStats.Current == nil {
		t.Fatal("Expected PidsStats to be set")
	}

	if len(actualStats.PidsStats.Current) != 1 {
		t.Fatal("Expected PidsStats.Current to have at least one element")
	}

	if actualStats.PidsStats.Max != configs.MaxPids {
		t.Fatal("Expected PidsStats.Max to return -1")
	}
}

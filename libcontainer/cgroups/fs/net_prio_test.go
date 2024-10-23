package fs

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
)

var prioMap = []*cgroups.IfPrioMap{
	{
		Interface: "test",
		Priority:  5,
	},
}

func TestNetPrioSetIfPrio(t *testing.T) {
	path := tempDir(t, "net_prio")

	r := &cgroups.Resources{
		NetPrioIfpriomap: prioMap,
	}
	netPrio := &NetPrioGroup{}
	if err := netPrio.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "net_prio.ifpriomap")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(value, "test 5") {
		t.Fatal("Got the wrong value, set net_prio.ifpriomap failed.")
	}
}

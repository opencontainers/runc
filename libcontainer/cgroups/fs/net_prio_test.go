// +build linux

package fs

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

var prioMap = []*configs.IfPrioMap{
	{
		Interface: "test",
		Priority:  5,
	},
}

func TestNetPrioSetIfPrio(t *testing.T) {
	helper := cgroups.NewCgroupTestUtil("net_prio", t)

	helper.CgroupData.Config.Resources.NetPrioIfpriomap = prioMap
	netPrio := &NetPrioGroup{}
	if err := netPrio.Set(helper.CgroupPath, helper.CgroupData.Config.Resources); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(helper.CgroupPath, "net_prio.ifpriomap")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(value, "test 5") {
		t.Fatal("Got the wrong value, set net_prio.ifpriomap failed.")
	}
}

// +build linux

package fs

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestFreezerSetState(t *testing.T) {
	helper := cgroups.NewCgroupTestUtil("freezer", t)

	helper.WriteFileContents(map[string]string{
		"freezer.state": string(configs.Frozen),
	})

	helper.CgroupData.Config.Resources.Freezer = configs.Thawed
	freezer := &FreezerGroup{}
	if err := freezer.Set(helper.CgroupPath, helper.CgroupData.Config.Resources); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(helper.CgroupPath, "freezer.state")
	if err != nil {
		t.Fatal(err)
	}
	if value != string(configs.Thawed) {
		t.Fatal("Got the wrong value, set freezer.state failed.")
	}
}

func TestFreezerSetInvalidState(t *testing.T) {
	helper := cgroups.NewCgroupTestUtil("freezer", t)

	const (
		invalidArg configs.FreezerState = "Invalid"
	)

	helper.CgroupData.Config.Resources.Freezer = invalidArg
	freezer := &FreezerGroup{}
	if err := freezer.Set(helper.CgroupPath, helper.CgroupData.Config.Resources); err == nil {
		t.Fatal("Failed to return invalid argument error")
	}
}

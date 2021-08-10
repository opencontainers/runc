package fs

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestFreezerSetState(t *testing.T) {
	path := tempDir(t, "freezer")

	writeFileContents(t, path, map[string]string{
		"freezer.state": string(configs.Frozen),
	})

	r := &configs.Resources{
		Freezer: configs.Thawed,
	}
	freezer := &FreezerGroup{}
	if err := freezer.Set(path, r); err != nil {
		t.Fatal(err)
	}

	value, err := fscommon.GetCgroupParamString(path, "freezer.state")
	if err != nil {
		t.Fatal(err)
	}
	if value != string(configs.Thawed) {
		t.Fatal("Got the wrong value, set freezer.state failed.")
	}
}

func TestFreezerSetInvalidState(t *testing.T) {
	path := tempDir(t, "freezer")

	const invalidArg configs.FreezerState = "Invalid"

	r := &configs.Resources{
		Freezer: invalidArg,
	}
	freezer := &FreezerGroup{}
	if err := freezer.Set(path, r); err == nil {
		t.Fatal("Failed to return invalid argument error")
	}
}

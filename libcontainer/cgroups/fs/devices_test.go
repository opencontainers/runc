package fs

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
)

func TestDevicesSetAllow(t *testing.T) {
	path := tempDir(t, "devices")

	writeFileContents(t, path, map[string]string{
		"devices.allow": "",
		"devices.deny":  "",
		"devices.list":  "a *:* rwm",
	})

	r := &configs.Resources{
		Devices: []*devices.Rule{
			{
				Type:        devices.CharDevice,
				Major:       1,
				Minor:       5,
				Permissions: devices.Permissions("rwm"),
				Allow:       true,
			},
		},
	}

	d := &DevicesGroup{TestingSkipFinalCheck: true}
	if err := d.Set(path, r); err != nil {
		t.Fatal(err)
	}

	// The default deny rule must be written.
	value, err := fscommon.GetCgroupParamString(path, "devices.deny")
	if err != nil {
		t.Fatal(err)
	}
	if value[0] != 'a' {
		t.Errorf("Got the wrong value (%q), set devices.deny failed.", value)
	}

	// Permitted rule must be written.
	if value, err := fscommon.GetCgroupParamString(path, "devices.allow"); err != nil {
		t.Fatal(err)
	} else if value != "c 1:5 rwm" {
		t.Errorf("Got the wrong value (%q), set devices.allow failed.", value)
	}
}

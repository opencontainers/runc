// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
)

type DevicesGroup struct {
}

func (s *DevicesGroup) Name() string {
	return "devices"
}

func (s *DevicesGroup) Apply(d *cgroupData) error {
	_, err := d.join("devices")
	if err != nil {
		// We will return error even it's `not found` error, devices
		// cgroup is hard requirement for container's security.
		return err
	}
	return nil
}

func (s *DevicesGroup) Set(path string, cgroup *configs.Cgroup) error {
	if system.RunningInUserNS() {
		return nil
	}

	// The devices list is a whitelist, so we must first deny all devices.
	// XXX: This is incorrect for device list updates as it will result in
	//      spurrious errors in the container, but we will solve that
	//      separately.
	if err := fscommon.WriteFile(path, "devices.deny", "a"); err != nil {
		return err
	}

	devices := cgroup.Resources.Devices
	for _, dev := range devices {
		file := "devices.deny"
		if dev.Allow {
			file = "devices.allow"
		}
		if err := fscommon.WriteFile(path, file, dev.CgroupString()); err != nil {
			return err
		}
	}
	return nil
}

func (s *DevicesGroup) Remove(d *cgroupData) error {
	return removePath(d.path("devices"))
}

func (s *DevicesGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

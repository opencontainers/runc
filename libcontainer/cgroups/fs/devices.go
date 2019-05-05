// +build linux

package fs

import (
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
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

	devList, err := readFile(path, "devices.list")
	if err != nil {
		return err
	}

	devices := cgroup.Resources.Devices
	if len(devices) > 0 {
		for _, dev := range devices {
			file := "devices.deny"
			//The first time:
			// 	1. write 'a *:* rwm' to devices.deny
			//  2. add all allowed devices in a loop
			//Any further updates(from k8s, or docker cli):
			//  1. skip 'deny all' procedure if it's not the first time. ( Check if 'a *:* rmw' exists or not)
			//  2. add all allowed devices to current devices.list (if already exists, nothing happen)

			if !dev.Allow && !strings.HasPrefix(devList, "a *:* rwm") && dev.CgroupString() == "a *:* rwm" {
				continue
			}
			if dev.Allow {
				file = "devices.allow"
			}
			if err := writeFile(path, file, dev.CgroupString()); err != nil {
				return err
			}
		}
		return nil
	}
	if cgroup.Resources.AllowAllDevices != nil {
		if *cgroup.Resources.AllowAllDevices == false {
			if strings.HasPrefix(devList, "a *:* rwm") {
				if err := writeFile(path, "devices.deny", "a"); err != nil {
					return err
				}
			}

			for _, dev := range cgroup.Resources.AllowedDevices {
				if err := writeFile(path, "devices.allow", dev.CgroupString()); err != nil {
					return err
				}
			}
			return nil
		}

		if err := writeFile(path, "devices.allow", "a"); err != nil {
			return err
		}
	}

	for _, dev := range cgroup.Resources.DeniedDevices {
		if err := writeFile(path, "devices.deny", dev.CgroupString()); err != nil {
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

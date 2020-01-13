// +build linux

package fs

import (
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/fscommon"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"
)

type DevicesGroup struct {
}

type Empty struct{}

var (
	defaultDevice = &configs.Device{Type: 'a', Major: -1, Minor: -1, Permissions: "rwm"}
)

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

	devices := cgroup.Resources.Devices
	oldAllowedDevices, err := readDevicesExceptDefault(path)
	if err != nil {
		return err
	}

	if len(devices) > 0 {
		for _, dev := range devices {
			file := "devices.deny"
			if dev.Allow {
				file = "devices.allow"
			}

			// For the second time set, we don't deny all devices
			if dev.Type == defaultDevice.Type && len(oldAllowedDevices) != 0 {
				file = ""
			}

			if len(file) > 0 {
				if err := fscommon.WriteFile(path, file, dev.CgroupString()); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if cgroup.Resources.AllowAllDevices != nil {
		if *cgroup.Resources.AllowAllDevices == false {
			// For the first time set, we deny all devices to initialize the cgroup
			if len(oldAllowedDevices) == 0 {
				if err := fscommon.WriteFile(path, "devices.deny", "a"); err != nil {
					return err
				}
			}

			newAllowedDevices := make(map[string]Empty)
			for _, dev := range cgroup.AllowedDevices {
				newAllowedDevices[dev.CgroupString()] = Empty{}
			}

			// Deny no longer allowed devices
			for cgroupString := range oldAllowedDevices {
				if _, found := newAllowedDevices[cgroupString]; !found {
					if err := fscommon.WriteFile(path, "devices.deny", cgroupString); err != nil {
						return err
					}
				}
			}

			// Allow new devices
			for cgroupString := range newAllowedDevices {
				if _, found := oldAllowedDevices[cgroupString]; !found {
					if err := fscommon.WriteFile(path, "devices.allow", cgroupString); err != nil {
						return err
					}
				}
			}

			return nil
		}

		if err := fscommon.WriteFile(path, "devices.allow", "a"); err != nil {
			return err
		}
	}

	for _, dev := range cgroup.Resources.DeniedDevices {
		if err := fscommon.WriteFile(path, "devices.deny", dev.CgroupString()); err != nil {
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

func readDevicesExceptDefault(path string) (allowed map[string]Empty, err error) {
	cgroupData, err := fscommon.ReadFile(path, "devices.list")
	if err != nil {
		return nil, err
	}

	allowedDevices := make(map[string]Empty)
	defaultDeviceString := defaultDevice.CgroupString()
	for _, data := range strings.Split(cgroupData, "\n") {
		// skip allow all devices
		if len(data) == 0 || data == defaultDeviceString {
			continue
		}
		allowedDevices[data] = Empty{}
	}

	return allowedDevices, nil
}

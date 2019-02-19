// +build linux

package fs

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/system"

	systemdDbus "github.com/coreos/go-systemd/dbus"
	"github.com/godbus/dbus"
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

	devices := cgroup.Resources.Devices
	if len(devices) > 0 {
		for _, dev := range devices {
			file := "devices.deny"
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
			if err := writeFile(path, "devices.deny", "a"); err != nil {
				return err
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

	return s.SetCgroupv1(path, cgroup)
}

func (s *DevicesGroup) SetCgroupv1(path string, cgroup *configs.Cgroup) error {
	return nil
}

type deviceAllow struct {
	Path        string
	Permissions string
}

func (s *DevicesGroup) ToSystemdProperties(cgroup *configs.Cgroup) ([]systemdDbus.Property, error) {
	var devAllows []deviceAllow
	devPolicy := "strict"

	devices := cgroup.Resources.Devices
	if len(devices) > 0 {
		blockedAll := false
		for _, dev := range devices {
			if !blockedAll {
				// Expect the first rule to block all, in which
				// case we can translate this cgroup config to
				// something systemd will understand.
				if dev.Type == 'a' && !dev.Allow {
					blockedAll = true
				} else {
					return []systemdDbus.Property{}, fmt.Errorf("systemd only supports a whitelist on device cgroup, please use AllowedDevices instead.")
				}
				continue
			}
			// Ok, now we're handling the second+ device rules to
			// whitelist the items that matter to us.
			if !dev.Allow {
				// We already blocked all, so continue...
				continue
			}
			if devPath := dev.SystemdCgroupPath(); devPath != "" {
				devAllows = append(devAllows, deviceAllow{
					Path:        devPath,
					Permissions: dev.Permissions,
				})
			}
		}
	} else if cgroup.Resources.AllowAllDevices != nil {
		if *cgroup.Resources.AllowAllDevices {
			devPolicy = "auto"
		} else {
			for _, dev := range cgroup.Resources.AllowedDevices {
				if devPath := dev.SystemdCgroupPath(); devPath != "" {
					devAllows = append(devAllows, deviceAllow{
						Path:        devPath,
						Permissions: dev.Permissions,
					})
				}
			}
		}
	}
	return []systemdDbus.Property{
		{
			Name:  "DevicePolicy",
			Value: dbus.MakeVariant(devPolicy),
		},
		{
			Name:  "DeviceAllow",
			Value: dbus.MakeVariant(devAllows),
		},
	}, nil
}

func (s *DevicesGroup) Remove(d *cgroupData) error {
	return removePath(d.path("devices"))
}

func (s *DevicesGroup) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

// +build linux

package fs

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/cgroups/ebpf"
	"github.com/opencontainers/runc/libcontainer/cgroups/ebpf/devicefilter"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type DevicesGroupV2 struct {
}

func (s *DevicesGroupV2) Name() string {
	return "devices"
}

func (s *DevicesGroupV2) Apply(d *cgroupData) error {
	return nil
}

func isRWM(cgroupPermissions string) bool {
	r := false
	w := false
	m := false
	for _, rn := range cgroupPermissions {
		switch rn {
		case 'r':
			r = true
		case 'w':
			w = true
		case 'm':
			m = true
		}
	}
	return r && w && m
}

// the logic is from crun
// https://github.com/containers/crun/blob/0.10.2/src/libcrun/cgroup.c#L1644-L1652
func canSkipEBPFError(cgroup *configs.Cgroup) bool {
	for _, dev := range cgroup.Resources.Devices {
		if dev.Allow || !isRWM(dev.Permissions) {
			return false
		}
	}
	return true
}

func (s *DevicesGroupV2) Set(path string, cgroup *configs.Cgroup) error {
	if cgroup.Resources.AllowAllDevices != nil {
		// never set by OCI specconv
		return errors.New("libcontainer AllowAllDevices is not supported, use Devices")
	}
	if len(cgroup.Resources.DeniedDevices) != 0 {
		// never set by OCI specconv
		return errors.New("libcontainer DeniedDevices is not supported, use Devices")
	}
	insts, license, err := devicefilter.DeviceFilter(cgroup.Devices)
	if err != nil {
		return err
	}
	dirFD, err := unix.Open(path, unix.O_DIRECTORY|unix.O_RDONLY, 0600)
	if err != nil {
		return errors.Errorf("cannot get dir FD for %s", path)
	}
	defer unix.Close(dirFD)
	if _, err := ebpf.LoadAttachCgroupDeviceFilter(insts, license, dirFD); err != nil {
		if !canSkipEBPFError(cgroup) {
			return err
		}
	}
	return nil
}

func (s *DevicesGroupV2) Remove(d *cgroupData) error {
	return nil
}

func (s *DevicesGroupV2) GetStats(path string, stats *cgroups.Stats) error {
	return nil
}

// +build linux

package fs

import (
	"fmt"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type IntelRdtGroup struct {
}

func (s *IntelRdtGroup) Name() string {
	return "intel_rdt"
}

func (s *IntelRdtGroup) Apply(d *cgroupData) error {
	dir, err := d.join("intel_rdt")
	if err != nil {
		if !cgroups.IsNotFound(err) {
			return err
		}
		// We will not return err here when:
		// 1. The h/w platform doesn't support Intel RDT/CAT feature,
		//    intel_rdt cgroup is not enabled in kernel.
		// 2. intel_rdt cgroup is not mounted
		return nil
	}

	if err := s.Set(dir, d.config); err != nil {
		return err
	}

	return nil
}

func (s *IntelRdtGroup) Set(path string, cgroup *configs.Cgroup) error {
	// The valid CBM (capacity bitmask) is a *contiguous bits set* and
	// number of bits that can be set is less than the max bit. The max
	// bits in the CBM is varied among supported Intel platforms.
	//
	// By default the child cgroups inherit the CBM from parent. The CBM
	// in a child cgroup should be a subset of the CBM in parent. Kernel
	// will check if it is valid when writing.
	//
	// e.g., 0xfffff in root cgroup indicates the max bits of CBM is 20
	// bits, which mapping to entire L3 cache capacity. Some valid CBM
	// values to Set in children cgroup: 0xf, 0xf0, 0x3ff, 0x1f00 and etc.
	if cgroup.Resources.IntelRdtL3Cbm != 0 {
		l3CbmStr := fmt.Sprintf("0x%s", strconv.FormatUint(cgroup.Resources.IntelRdtL3Cbm, 16))
		if err := writeFile(path, "intel_rdt.l3_cbm", l3CbmStr); err != nil {
			return err
		}
	}

	return nil
}

func (s *IntelRdtGroup) Remove(d *cgroupData) error {
	return removePath(d.path("intel_rdt"))
}

func (s *IntelRdtGroup) GetStats(path string, stats *cgroups.Stats) error {
	value, err := getCgroupParamUintHex(path, "intel_rdt.l3_cbm")
	if err != nil {
		return fmt.Errorf("failed to parse intel_rdt.l3_cbm - %s", err)
	}

	stats.IntelRdtStats.L3Cbm = value

	return nil
}

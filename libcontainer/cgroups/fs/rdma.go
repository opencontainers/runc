// +build linux

package fs

import (
	"fmt"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

type RdmaGroup struct {
}

func (s *RdmaGroup) Name() string {
	return "rdma"
}

func (s *RdmaGroup) Apply(d *cgroupData) error {
	_, err := d.join("rdma")
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *RdmaGroup) Set(path string, cgroup *configs.Cgroup) error {
	for _, rdmaLimit := range cgroup.Resources.RdmaLimits {
		var handleLimit, objectLimit string

		if rdmaLimit.HcaHandleLimit == -1 {
			handleLimit = "max"
		} else {
			handleLimit = strconv.FormatInt(rdmaLimit.HcaHandleLimit, 10)
		}

		if rdmaLimit.HcaObjectLimit == -1 {
			objectLimit = "max"
		} else {
			objectLimit = strconv.FormatInt(rdmaLimit.HcaObjectLimit, 10)
		}

		if err := writeFile(path, "rdma.current", fmt.Sprintf("%s hca_handle=%s hca_object=%s", rdmaLimit.InterfaceName, handleLimit, objectLimit)); err != nil {
			return err
		}
	}

	return nil
}

func (s *RdmaGroup) Remove(d *cgroupData) error {
	return removePath(d.path("rdma"))
}

func (s *RdmaGroup) GetStats(path string, stats *cgroups.Stats) error {
	str, err := readFile(path, "rdma.current")
	if err != nil {
		return err
	}

	var interfaceName string
	var rdmaStats cgroups.RdmaStats
	fmt.Sscanf(str, "%s hca_handle=%d hca_object=%d", interfaceName, rdmaStats.HcaHandle, rdmaStats.HcaObject)
	stats.RdmaStats[interfaceName] = rdmaStats
	return nil
}


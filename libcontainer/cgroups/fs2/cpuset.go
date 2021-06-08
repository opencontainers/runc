package fs2

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func isCPUSetSet(r *configs.Resources) bool {
	return r.CPUSetCPUs != "" || r.CPUSetMems != ""
}

func setCPUSet(dirPath string, r *configs.Resources) error {
	if !isCPUSetSet(r) {
		return nil
	}

	if r.CPUSetCPUs != "" {
		if err := cgroups.WriteFile(dirPath, "cpuset.cpus", r.CPUSetCPUs); err != nil {
			return err
		}
	}
	if r.CPUSetMems != "" {
		if err := cgroups.WriteFile(dirPath, "cpuset.mems", r.CPUSetMems); err != nil {
			return err
		}
	}
	return nil
}

package libcontainer

import (
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/network"
)

// Returns all available stats for the given container.
func GetContainerStats(container *Config, runtimeCkpt *RuntimeCkpt) (*ContainerStats, error) {
	containerStats := NewContainerStats()
	stats, err := fs.GetStats(container.Cgroups)
	if err != nil {
		return containerStats, err
	}
	containerStats.CgroupStats = stats
	networkStats, err := network.GetStats(&runtimeCkpt.NetworkCkpt)
	if err != nil {
		return containerStats, err
	}
	containerStats.NetworkStats = networkStats

	return containerStats, nil
}

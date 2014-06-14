package api

import (
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/cgroups/fs"

	"github.com/docker/libcontainer/network"
)

// Returns all available stats for the given container.
func GetContainerStats(container *libcontainer.Container, networkInfo *network.NetworkRuntimeInfo) (*ContainerStats, error) {
	containerStats := NewContainerStats()
	stats, err := fs.GetStats(container.Cgroups)
	if err != nil {
		return containerStats, err
	}
	containerStats.CgroupStats = stats
	networkStats, err := network.GetStats(networkInfo)
	if err != nil {
		return containerStats, err
	}
	containerStats.NetworkStats = networkStats

	return containerStats, nil
}

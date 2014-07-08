/*
Temporary API endpoint for libcontainer while the full API is finalized (api.go).
*/
package libcontainer

import (
	"github.com/docker/libcontainer/cgroups/fs"
	"github.com/docker/libcontainer/network"
)

// TODO(vmarmol): Complete Stats() in final libcontainer API and move users to that.
// DEPRECATED: The below portions are only to be used during the transition to the official API.
// Returns all available stats for the given container.
func GetStats(container *Config, state *State) (*ContainerStats, error) {
	var containerStats ContainerStats
	stats, err := fs.GetStats(container.Cgroups)
	if err != nil {
		return &containerStats, err
	}
	containerStats.CgroupStats = stats
	networkStats, err := network.GetStats(&state.NetworkState)
	if err != nil {
		return &containerStats, err
	}
	containerStats.NetworkStats = networkStats

	return &containerStats, nil
}

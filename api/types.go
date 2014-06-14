package api

import (
	"github.com/docker/libcontainer/cgroups"
	"github.com/docker/libcontainer/network"
)

type ContainerStats struct {
	NetworkStats network.NetworkStats `json:"network_stats, omitempty"`
	CgroupStats  *cgroups.Stats       `json:"cgroup_stats, omitempty"`
}

func NewContainerStats() *ContainerStats {
	cgroupStats := cgroups.NewStats()
	return &ContainerStats{CgroupStats: cgroupStats}
}

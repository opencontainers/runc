package libcontainer

import (
	"github.com/docker/libcontainer/mount"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/security/capabilities"
)

func GetInternalMountConfig(container *Container) *mount.MountConfig {
	out := &mount.MountConfig{
		NoPivotRoot: container.NoPivotRoot,
		ReadonlyFs:  container.ReadonlyFs,
		DeviceNodes: container.DeviceNodes,
		MountLabel:  container.Context["mount_label"],
		Mounts:      (mount.Mounts)(container.Mounts),
	}
	return out
}

func GetInternalNetworkConfig(net *Network) *network.Network {
	return &network.Network{
		Type:       net.Type,
		NsPath:     net.Context["nspath"],
		Bridge:     net.Context["bridge"],
		VethPrefix: net.Context["prefix"],
		Address:    net.Address,
		Gateway:    net.Gateway,
		Mtu:        net.Mtu,
	}
}

func GetAllCapabilities() []string {
	return capabilities.GetAllCapabilities()
}

func DropBoundingSet(container *Container) error {
	return capabilities.DropBoundingSet(container.Capabilities)
}

func DropCapabilities(container *Container) error {
	return capabilities.DropCapabilities(container.Capabilities)
}

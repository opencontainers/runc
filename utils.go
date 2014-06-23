package libcontainer

import (
	"github.com/docker/libcontainer/security/capabilities"
)

func GetAllCapabilities() []string {
	return capabilities.GetAllCapabilities()
}

func DropBoundingSet(container *Container) error {
	return capabilities.DropBoundingSet(container.Capabilities)
}

func DropCapabilities(container *Container) error {
	return capabilities.DropCapabilities(container.Capabilities)
}

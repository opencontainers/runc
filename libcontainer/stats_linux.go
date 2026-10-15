package libcontainer

import (
	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runc/types"
)

type Stats struct {
	// Deprecated: always empty. Network statistics should be obtained
	// by whoever sets up the container network.
	Interfaces    []*types.NetworkInterface
	CgroupStats   *cgroups.Stats
	IntelRdtStats *intelrdt.Stats
}

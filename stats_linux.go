package libcontainer

import "github.com/opencontainers/libcontainer/cgroups"

type Stats struct {
	Interfaces  []*NetworkInterface
	CgroupStats *cgroups.Stats
}

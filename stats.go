package libcontainer

import "github.com/docker/libcontainer/cgroups"

type Stats struct {
	Interfaces  []*networkInterface
	CgroupStats *cgroups.Stats
}

type networkInterface struct {
	// Name is the name of the network interface.
	Name string

	RxBytes   uint64
	RxPackets uint64
	RxErrors  uint64
	RxDropped uint64
	TxBytes   uint64
	TxPackets uint64
	TxErrors  uint64
	TxDropped uint64
}

package configs

import "golang.org/x/sys/unix"

// Memory policy modes and flags as defined in /usr/include/linux/mempolicy.h

//nolint:revive,staticcheck,nolintlint // ignore ALL_CAPS errors in consts from numaif.h, will match unix.* in the future
const (
	MPOL_DEFAULT             = 0
	MPOL_PREFERRED           = 1
	MPOL_BIND                = 2
	MPOL_INTERLEAVE          = 3
	MPOL_LOCAL               = 4
	MPOL_PREFERRED_MANY      = 5
	MPOL_WEIGHTED_INTERLEAVE = 6

	MPOL_F_STATIC_NODES   = 1 << 15
	MPOL_F_RELATIVE_NODES = 1 << 14
	MPOL_F_NUMA_BALANCING = 1 << 13
)

// LinuxMemoryPolicy contains memory policy configuration.
type LinuxMemoryPolicy struct {
	// Mode specifies memory policy mode without mode flags. See
	// set_mempolicy() documentation for details.
	Mode uint `json:"mode,omitempty"`
	// Flags contains mode flags.
	Flags uint `json:"flags,omitempty"`
	// Nodes contains NUMA nodes to which the mode applies.
	Nodes *unix.CPUSet `json:"nodes,omitempty"`
}

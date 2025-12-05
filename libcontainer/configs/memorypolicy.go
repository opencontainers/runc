package configs

import "golang.org/x/sys/unix"

// Memory policy modes and flags as defined in /usr/include/linux/mempolicy.h
//
// Deprecated: use constants from [unix] instead.
//
//nolint:revive,staticcheck,nolintlint // ignore ALL_CAPS errors in consts from numaif.h, will match unix.* in the future
const (
	MPOL_DEFAULT             = unix.MPOL_DEFAULT
	MPOL_PREFERRED           = unix.MPOL_PREFERRED
	MPOL_BIND                = unix.MPOL_BIND
	MPOL_INTERLEAVE          = unix.MPOL_INTERLEAVE
	MPOL_LOCAL               = unix.MPOL_LOCAL
	MPOL_PREFERRED_MANY      = unix.MPOL_PREFERRED_MANY
	MPOL_WEIGHTED_INTERLEAVE = unix.MPOL_WEIGHTED_INTERLEAVE

	MPOL_F_STATIC_NODES   = unix.MPOL_F_STATIC_NODES
	MPOL_F_RELATIVE_NODES = unix.MPOL_F_RELATIVE_NODES
	MPOL_F_NUMA_BALANCING = unix.MPOL_F_NUMA_BALANCING
)

// LinuxMemoryPolicy contains memory policy configuration.
type LinuxMemoryPolicy struct {
	// Mode specifies memory policy mode without mode flags. See
	// set_mempolicy() documentation for details.
	Mode int `json:"mode,omitempty"`
	// Flags contains mode flags.
	Flags int `json:"flags,omitempty"`
	// Nodes contains NUMA nodes to which the mode applies.
	Nodes *unix.CPUSet `json:"nodes,omitempty"`
}

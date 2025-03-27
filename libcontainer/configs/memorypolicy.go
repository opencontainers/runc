package configs

import "golang.org/x/sys/unix"

//nolint:revive,staticcheck,nolintlint // ignore ALL_CAPS errors in consts from numaif.h, will match unix.* in the future
const (
	MPOL_DEFAULT = iota
	MPOL_PREFERRED
	MPOL_BIND
	MPOL_INTERLEAVE
	MPOL_LOCAL
	MPOL_PREFERRED_MANY
	MPOL_WEIGHTED_INTERLEAVE

	MPOL_F_STATIC_NODES   = 1 << 15
	MPOL_F_RELATIVE_NODES = 1 << 14
	MPOL_F_NUMA_BALANCING = 1 << 13
)

// LinuxMemoryPolicy contains memory policy configuration.
type LinuxMemoryPolicy struct {
	// Mode specifies memory policy mode without mode flags. See
	// set_mempolicy() documentation for details.
	Mode uint
	// Flags contains mode flags.
	Flags []uint
	// Nodes contains NUMA nodes to which the mode applies.
	Nodes *unix.CPUSet
}

package configs

// LinuxMemoryPolicy contains memory policy configuration.
type LinuxMemoryPolicy struct {
	// Mode combines memory poliy mode and mode flags. Refer to
	// set_mempolicy() documentation for details.
	Mode uint
	// Contains NUMA nodes to which the mode applies.
	Nodes []int
}

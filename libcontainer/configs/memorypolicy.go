package configs

import "golang.org/x/sys/unix"

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

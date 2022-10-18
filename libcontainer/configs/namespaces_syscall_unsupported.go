//go:build !linux && !windows
// +build !linux,!windows

package configs

func (n *Namespace) Syscall() int {
	panic("No namespace syscall support")
}

// CloneFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare. This function returns flags only for new namespaces.
func (n *Namespaces) CloneFlags() uintptr {
	panic("No namespace syscall support")
}

// NonCloneFlags parses the container's Namespaces options that are not
// related to clone() or unshare() system calls. This function returns
// flags only for new namespaces.
func (n *Namespaces) NonCloneFlags() uintptr {
	panic("No namespace syscall support")
}

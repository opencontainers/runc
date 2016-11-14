// +build linux

package configs

import "syscall"

func (n *Namespace) Syscall() int {
	return namespaceInfo[n.Type]
}

// This is not yet in the Go stdlib.
const syscall_CLONE_NEWCGROUP = (1 << 25)

var namespaceInfo = map[NamespaceType]int{
	NEWNET:    syscall.CLONE_NEWNET,
	NEWNS:     syscall.CLONE_NEWNS,
	NEWUSER:   syscall.CLONE_NEWUSER,
	NEWIPC:    syscall.CLONE_NEWIPC,
	NEWUTS:    syscall.CLONE_NEWUTS,
	NEWPID:    syscall.CLONE_NEWPID,
	NEWCGROUP: syscall_CLONE_NEWCGROUP,
}

// CloneFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare. This function returns flags only for new namespaces.
func (n *Namespaces) CloneFlags() uintptr {
	var flag int
	for _, v := range *n {
		if v.Path != "" {
			continue
		}
		flag |= namespaceInfo[v.Type]
	}
	return uintptr(flag)
}

//go:build linux
// +build linux

package configs

import "golang.org/x/sys/unix"

func (n *Namespace) Syscall() int {
	return namespaceCloneInfo[n.Type]
}

var namespaceCloneInfo = map[NamespaceType]int{
	NEWNET:    unix.CLONE_NEWNET,
	NEWNS:     unix.CLONE_NEWNS,
	NEWUSER:   unix.CLONE_NEWUSER,
	NEWIPC:    unix.CLONE_NEWIPC,
	NEWUTS:    unix.CLONE_NEWUTS,
	NEWPID:    unix.CLONE_NEWPID,
	NEWCGROUP: unix.CLONE_NEWCGROUP,
}

// CloneFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare. This function returns flags only for new namespaces.
func (n *Namespaces) CloneFlags() uintptr {
	var flag int
	for _, v := range *n {
		if v.Path != "" {
			continue
		}
		flag |= namespaceCloneInfo[v.Type]
	}
	return uintptr(flag)
}

// NonCloneFlags parses the container's Namespaces options that are not
// related to clone() or unshare() system calls. This function returns
// flags only for new namespaces.
func (n *Namespaces) NonCloneFlags() uintptr {
	var flag uint64
	for _, v := range *n {
		if v.Path != "" {
			continue
		}
		flag |= v.parseNonCloneFlags()
	}
	return uintptr(flag)
}

func (n *Namespace) parseNonCloneFlags() uint64 {
	var flag uint64
	switch n.Type {
	case NEWIMA:
		flag |= 0x400000000
	}
	return flag
}

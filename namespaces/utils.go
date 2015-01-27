// +build linux

package namespaces

import (
	"os"
	"syscall"

	"github.com/docker/libcontainer/configs"
)

type initError struct {
	Message string `json:"message,omitempty"`
}

func (i initError) Error() string {
	return i.Message
}

var namespaceInfo = map[configs.NamespaceType]int{
	configs.NEWNET:  syscall.CLONE_NEWNET,
	configs.NEWNS:   syscall.CLONE_NEWNS,
	configs.NEWUSER: syscall.CLONE_NEWUSER,
	configs.NEWIPC:  syscall.CLONE_NEWIPC,
	configs.NEWUTS:  syscall.CLONE_NEWUTS,
	configs.NEWPID:  syscall.CLONE_NEWPID,
}

// New returns a newly initialized Pipe for communication between processes
func newInitPipe() (parent *os.File, child *os.File, err error) {
	fds, err := syscall.Socketpair(syscall.AF_LOCAL, syscall.SOCK_STREAM|syscall.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	return os.NewFile(uintptr(fds[1]), "parent"), os.NewFile(uintptr(fds[0]), "child"), nil
}

// GetNamespaceFlags parses the container's Namespaces options to set the correct
// flags on clone, unshare. This functions returns flags only for new namespaces.
func GetNamespaceFlags(namespaces configs.Namespaces) (flag int) {
	for _, v := range namespaces {
		if v.Path != "" {
			continue
		}
		flag |= namespaceInfo[v.Type]
	}
	return flag
}

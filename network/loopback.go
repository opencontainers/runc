// +build linux

package network

import (
	"fmt"

	"github.com/docker/libcontainer/configs"
)

// Loopback is a network strategy that provides a basic loopback device
type Loopback struct {
}

func (l *Loopback) Create(n *configs.Network, nspid int) error {
	return nil
}

func (l *Loopback) Initialize(config *configs.Network) error {
	// Do not set the MTU on the loopback interface - use the default.
	if err := InterfaceUp("lo"); err != nil {
		return fmt.Errorf("lo up %s", err)
	}
	return nil
}

// +build linux

package network

import (
	"fmt"

	"github.com/docker/libcontainer/configs"
)

// Veth is a network strategy that uses a bridge and creates
// a veth pair, one that stays outside on the host and the other
// is placed inside the container's namespace
type Veth struct {
}

const defaultDevice = "eth0"

func (v *Veth) Create(n *configs.Network, nspid int) error {
	var (
		bridge     = n.Bridge
		txQueueLen = n.TxQueueLen
	)
	if bridge == "" {
		return fmt.Errorf("bridge is not specified")
	}
	if err := CreateVethPair(n.VethHost, n.VethChild, txQueueLen); err != nil {
		return err
	}
	if err := SetInterfaceMaster(n.VethHost, bridge); err != nil {
		return err
	}
	if err := SetMtu(n.VethHost, n.Mtu); err != nil {
		return err
	}
	if err := InterfaceUp(n.VethHost); err != nil {
		return err
	}
	return SetInterfaceInNamespacePid(n.VethChild, nspid)
	return nil
}

func (v *Veth) Initialize(config *configs.Network) error {
	vethChild := config.VethChild
	if vethChild == "" {
		return fmt.Errorf("vethChild is not specified")
	}
	if err := InterfaceDown(vethChild); err != nil {
		return fmt.Errorf("interface down %s %s", vethChild, err)
	}
	if err := ChangeInterfaceName(vethChild, defaultDevice); err != nil {
		return fmt.Errorf("change %s to %s %s", vethChild, defaultDevice, err)
	}
	if config.MacAddress != "" {
		if err := SetInterfaceMac(defaultDevice, config.MacAddress); err != nil {
			return fmt.Errorf("set %s mac %s", defaultDevice, err)
		}
	}
	if err := SetInterfaceIp(defaultDevice, config.Address); err != nil {
		return fmt.Errorf("set %s ip %s", defaultDevice, err)
	}
	if config.IPv6Address != "" {
		if err := SetInterfaceIp(defaultDevice, config.IPv6Address); err != nil {
			return fmt.Errorf("set %s ipv6 %s", defaultDevice, err)
		}
	}

	if err := SetMtu(defaultDevice, config.Mtu); err != nil {
		return fmt.Errorf("set %s mtu to %d %s", defaultDevice, config.Mtu, err)
	}
	if err := InterfaceUp(defaultDevice); err != nil {
		return fmt.Errorf("%s up %s", defaultDevice, err)
	}
	if config.Gateway != "" {
		if err := SetDefaultGateway(config.Gateway, defaultDevice); err != nil {
			return fmt.Errorf("set gateway to %s on device %s failed with %s", config.Gateway, defaultDevice, err)
		}
	}
	if config.IPv6Gateway != "" {
		if err := SetDefaultGateway(config.IPv6Gateway, defaultDevice); err != nil {
			return fmt.Errorf("set gateway for ipv6 to %s on device %s failed with %s", config.IPv6Gateway, defaultDevice, err)
		}
	}
	return nil
}

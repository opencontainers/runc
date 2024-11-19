package libcontainer

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/types"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

var strategies = map[string]networkStrategy{
	"loopback": &loopback{},
}

// networkStrategy represents a specific network configuration for
// a container's networking stack
type networkStrategy interface {
	create(*network, int) error
	initialize(*network) error
	detach(*configs.Network) error
	attach(*configs.Network) error
}

// getStrategy returns the specific network strategy for the
// provided type.
func getStrategy(tpe string) (networkStrategy, error) {
	s, exists := strategies[tpe]
	if !exists {
		return nil, fmt.Errorf("unknown strategy type %q", tpe)
	}
	return s, nil
}

// Returns the network statistics for the network interfaces represented by the NetworkRuntimeInfo.
func getNetworkInterfaceStats(interfaceName string) (*types.NetworkInterface, error) {
	out := &types.NetworkInterface{Name: interfaceName}
	// This can happen if the network runtime information is missing - possible if the
	// container was created by an old version of libcontainer.
	if interfaceName == "" {
		return out, nil
	}
	type netStatsPair struct {
		// Where to write the output.
		Out *uint64
		// The network stats file to read.
		File string
	}
	// Ingress for host veth is from the container. Hence tx_bytes stat on the host veth is actually number of bytes received by the container.
	netStats := []netStatsPair{
		{Out: &out.RxBytes, File: "tx_bytes"},
		{Out: &out.RxPackets, File: "tx_packets"},
		{Out: &out.RxErrors, File: "tx_errors"},
		{Out: &out.RxDropped, File: "tx_dropped"},

		{Out: &out.TxBytes, File: "rx_bytes"},
		{Out: &out.TxPackets, File: "rx_packets"},
		{Out: &out.TxErrors, File: "rx_errors"},
		{Out: &out.TxDropped, File: "rx_dropped"},
	}
	for _, netStat := range netStats {
		data, err := readSysfsNetworkStats(interfaceName, netStat.File)
		if err != nil {
			return nil, err
		}
		*(netStat.Out) = data
	}
	return out, nil
}

// Reads the specified statistics available under /sys/class/net/<EthInterface>/statistics
func readSysfsNetworkStats(ethInterface, statsFile string) (uint64, error) {
	data, err := os.ReadFile(filepath.Join("/sys/class/net", ethInterface, "statistics", statsFile))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(string(bytes.TrimSpace(data)), 10, 64)
}

// loopback is a network strategy that provides a basic loopback device
type loopback struct{}

func (l *loopback) create(n *network, nspid int) error {
	return nil
}

func (l *loopback) initialize(config *network) error {
	return netlink.LinkSetUp(&netlink.Device{LinkAttrs: netlink.LinkAttrs{Name: "lo"}})
}

func (l *loopback) attach(n *configs.Network) (err error) {
	return nil
}

func (l *loopback) detach(n *configs.Network) (err error) {
	return nil
}

// netnsAttach takes the network device referenced by name in the current network namespace
// and moves to the network namespace passed as a parameter. It also configure the
// network device inside the new network namespace with the passed parameters.
func netnsAttach(name string, nsPath string, device configs.LinuxNetDevice) error {
	logrus.Debugf("attaching network device %s with attrs %#v to network namespace %s", name, device, nsPath)
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on runtime namespace: %w", name, err)
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Index = link.Attrs().Index

	attrs.Name = name
	if device.Name != "" {
		attrs.Name = device.Name
	}

	attrs.MTU = link.Attrs().MTU
	if device.MTU > 0 {
		attrs.MTU = int(device.MTU)
	}

	attrs.HardwareAddr = link.Attrs().HardwareAddr
	if device.HardwareAddress != "" {
		attrs.HardwareAddr, err = net.ParseMAC(device.HardwareAddress)
		if err != nil {
			return err
		}
	}

	ns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("could not get network namespace from path %s for network device %s : %w", nsPath, name, err)
	}

	attrs.Namespace = netlink.NsFd(ns)

	// set the interface down before we change the address inside the network namespace
	err = netlink.LinkSetDown(link)
	if err != nil {
		return err
	}

	dev := &netlink.Device{
		LinkAttrs: attrs,
	}

	err = netlink.LinkModify(dev)
	if err != nil {
		return fmt.Errorf("could not modify network device %s : %w", name, err)
	}

	// to avoid golang problem with goroutines we create the socket in the
	// namespace and use it directly
	nhNs, err := netlink.NewHandleAt(ns)
	if err != nil {
		return err
	}

	nsLink, err := nhNs.LinkByName(dev.Name)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on namespace %s: %w", dev.Name, nsPath, err)
	}

	err = nhNs.LinkSetUp(nsLink)
	if err != nil {
		return fmt.Errorf("failt to set up interface %s on namespace %s: %w", nsLink.Attrs().Name, nsPath, err)
	}

	for _, address := range device.Addresses {
		addr, err := netlink.ParseAddr(address)
		if err != nil {
			return err
		}

		err = nhNs.AddrAdd(nsLink, addr)
		if err != nil {
			return err
		}
	}
	return nil
}

// netnsDettach takes the network device referenced by name in the passed network namespace
// and moves to the root network namespace, restoring the original name. It also sets down
// the network device to avoid conflict with existing network configuraiton.
func netnsDettach(name string, nsPath string, device configs.LinuxNetDevice) error {
	logrus.Debugf("dettaching network device %s with attrs %#v to network namespace %s", name, device, nsPath)
	ns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("could not get network namespace from path %s for network device %s : %w", nsPath, name, err)
	}
	// to avoid golang problem with goroutines we create the socket in the
	// namespace and use it directly
	nhNs, err := netlink.NewHandleAt(ns)
	if err != nil {
		return fmt.Errorf("could not get network namespace handle: %w", err)
	}

	devName := device.Name
	if devName == "" {
		devName = name
	}

	nsLink, err := nhNs.LinkByName(devName)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on namespace %s: %w", device.Name, nsPath, err)
	}

	// set the device down to avoid network conflicts
	// when it is restored to the original namespace
	err = nhNs.LinkSetDown(nsLink)
	if err != nil {
		return err
	}

	// restore the original name if it was renamed
	if device.Name != name {
		err = nhNs.LinkSetName(nsLink, name)
		if err != nil {
			return err
		}
	}

	rootNs, err := netns.Get()
	if err != nil {
		return err
	}
	defer rootNs.Close()

	err = nhNs.LinkSetNsFd(nsLink, int(netlink.NsFd(rootNs)))
	if err != nil {
		return fmt.Errorf("failed to restore original network namespace: %w", err)
	}
	return nil
}

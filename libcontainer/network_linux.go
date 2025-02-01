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
	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"

	"golang.org/x/sys/unix"
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

	// set the interface down to change the attributes safely
	err = netlink.LinkSetDown(link)
	if err != nil {
		return fmt.Errorf("fail to set link down: %w", err)
	}

	// do interface rename and namespace change in the same operation to avoid
	// posible conflicts with the inteface name.
	flags := unix.NLM_F_REQUEST | unix.NLM_F_ACK
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, flags)

	// Get a netlink socket in current namespace
	s, err := nl.GetNetlinkSocketAt(netns.None(), netns.None(), unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("could not get network namespace handle: %w", err)
	}
	req.Sockets = map[int]*nl.SocketHandle{
		unix.NETLINK_ROUTE: {Socket: s},
	}

	// set the interface index
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(link.Attrs().Index)
	req.AddData(msg)

	// set the interface name, rename if requested
	newName := name
	if device.Name != "" {
		newName = device.Name
	}
	nameData := nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(newName))
	req.AddData(nameData)

	// set the new network namespace
	ns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return fmt.Errorf("could not get network namespace from path %s for network device %s : %w", nsPath, name, err)
	}

	val := nl.Uint32Attr(uint32(ns))
	attr := nl.NewRtAttr(unix.IFLA_NET_NS_FD, val)
	req.AddData(attr)

	_, err = req.Execute(unix.NETLINK_ROUTE, 0)
	if err != nil {
		return fmt.Errorf("fail to move network device %s to network namespace %s: %w", name, nsPath, err)
	}

	// to avoid golang problem with goroutines we create the socket in the
	// namespace and use it directly
	nhNs, err := netlink.NewHandleAt(ns)
	if err != nil {
		return err
	}

	nsLink, err := nhNs.LinkByName(newName)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on namespace %s : %w", newName, nsPath, err)
	}

	// set hardware address if requested
	if device.HardwareAddress != "" {
		hwaddr, err := net.ParseMAC(device.HardwareAddress)
		if err != nil {
			return err
		}
		err = netlink.LinkSetHardwareAddr(nsLink, hwaddr)
		if err != nil {
			return fmt.Errorf("fail to set mac %s : %w", hwaddr.String(), err)
		}
	}

	// set MTU if requested
	if device.MTU > 0 {
		err = netlink.LinkSetMTU(nsLink, int(device.MTU))
		if err != nil {
			return fmt.Errorf("fail to set MTU %d : %w", device.MTU, err)
		}
	}

	err = nhNs.LinkSetUp(nsLink)
	if err != nil {
		return fmt.Errorf("failt to set up interface %s on namespace %s: %w", nsLink.Attrs().Name, nsPath, err)
	}

	for _, address := range device.Addresses {
		addr, err := netlink.ParseAddr(address)
		if err != nil {
			return fmt.Errorf("invalid IP address %s : %w", address, err)
		}

		err = nhNs.AddrAdd(nsLink, addr)
		if err != nil {
			return fmt.Errorf("fail to add address %s : %w", addr.String(), err)
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

	// set the new network namespace
	rootNs, err := netns.Get()
	if err != nil {
		return fmt.Errorf("could not get current network namespace handle: %w", err)
	}
	defer rootNs.Close()

	// check the interface name, it could have been renamed
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
		return fmt.Errorf("fail to set interface down: %w", err)
	}

	// do interface rename and namespace change in the same operation to avoid
	// posible conflicts with the inteface name.
	flags := unix.NLM_F_REQUEST | unix.NLM_F_ACK
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, flags)

	// Get a netlink socket in current namespace
	s, err := nl.GetNetlinkSocketAt(ns, rootNs, unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("could not get network namespace handle: %w", err)
	}
	req.Sockets = map[int]*nl.SocketHandle{
		unix.NETLINK_ROUTE: {Socket: s},
	}

	// set the interface index
	msg := nl.NewIfInfomsg(unix.AF_UNSPEC)
	msg.Index = int32(nsLink.Attrs().Index)
	req.AddData(msg)

	// restore the original name if it was renamed
	if nsLink.Attrs().Name != name {
		nameData := nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name))
		req.AddData(nameData)
	}

	val := nl.Uint32Attr(uint32(rootNs))
	attr := nl.NewRtAttr(unix.IFLA_NET_NS_FD, val)
	req.AddData(attr)

	_, err = req.Execute(unix.NETLINK_ROUTE, 0)
	if err != nil {
		return fmt.Errorf("fail to move back interface to current namespace: %w", err)
	}

	return nil
}

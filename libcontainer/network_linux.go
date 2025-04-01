package libcontainer

import (
	"bytes"
	"fmt"
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

// devChangeNetNamespace allows to change the device name from a network namespace and optionally replace the existing name.
// This function ensures that the move and rename operations occur simultaneously.
// It preserves existing interface attributes, including IP addresses.
func devChangeNetNamespace(name string, nsPath string, device configs.LinuxNetDevice) error {
	logrus.Debugf("attaching network device %s with attrs %+v to network namespace %s", name, device, nsPath)
	link, err := netlink.LinkByName(name)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on runtime namespace: %w", name, err)
	}

	// set the interface down to change the attributes safely
	err = netlink.LinkSetDown(link)
	if err != nil {
		return fmt.Errorf("fail to set link down: %w", err)
	}

	// get the existing IP addresses
	addresses, err := netlink.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("fail to get ip addresses: %w", err)
	}

	// do interface rename and namespace change in the same operation to avoid
	// possible conflicts with the interface name.
	flags := unix.NLM_F_REQUEST | unix.NLM_F_ACK
	req := nl.NewNetlinkRequest(unix.RTM_NEWLINK, flags)

	// Get a netlink socket in current namespace
	s, err := nl.GetNetlinkSocketAt(netns.None(), netns.None(), unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("could not get network namespace handle: %w", err)
	}
	defer s.Close()

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
	defer ns.Close()

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
	defer nhNs.Close()

	nsLink, err := nhNs.LinkByName(newName)
	if err != nil {
		return fmt.Errorf("link not found for interface %s on namespace %s : %w", newName, nsPath, err)
	}

	for _, address := range addresses {
		// Only move permanent IP addresses configured by the user, dynamic addresses are excluded because
		// their validity may rely on the original network namespace's context and they may have limited
		// lifetimes and are not guaranteed to be available in a new namespace.
		// Ref: https://www.ietf.org/rfc/rfc3549.txt
		if address.Flags&unix.IFA_F_PERMANENT == 0 {
			continue
		}
		// Only move IP addresses with global scope because those are not host-specific, auto-configured,
		// or have limited network scope, making them unsuitable inside the container namespace.
		// Ref: https://www.ietf.org/rfc/rfc3549.txt
		if address.Scope != unix.RT_SCOPE_UNIVERSE {
			continue
		}
		// remove the interface attribute of the original address
		// to avoid issues when the interface is renamed.
		err = nhNs.AddrAdd(nsLink, &netlink.Addr{IPNet: address.IPNet})
		if err != nil {
			return fmt.Errorf("fail to set up address %s on namespace %s: %w", address.String(), nsPath, err)
		}
	}

	err = nhNs.LinkSetUp(nsLink)
	if err != nil {
		return fmt.Errorf("fail to set up interface %s on namespace %s: %w", nsLink.Attrs().Name, nsPath, err)
	}

	return nil
}

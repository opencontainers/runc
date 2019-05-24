// +build linux

package libcontainer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/vishvananda/netlink"
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
func getNetworkInterfaceStats(interfaceName, nicPath string) (*NetworkInterface, error) {
	out := &NetworkInterface{Name: interfaceName}
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
		data, err := readSysfsNetworkStats(interfaceName, netStat.File, nicPath)
		if err != nil {
			return nil, err
		}
		*(netStat.Out) = data
	}
	return out, nil
}

// Returns the network statistics for all network interfaces except loopback
func getAllInterfaceStats(initPid int) ([]*NetworkInterface, error) {
	var interfaces []*NetworkInterface
	// set the nic path
	nicPath := filepath.Join("/proc", strconv.Itoa(initPid), "root/sys/class/net")
	// get all the interfaces
	dirs, err := ioutil.ReadDir(nicPath)
	if err != nil {
		return interfaces, err
	}
	for _, dir := range dirs {
		interfaceName := dir.Name()
		filename := filepath.Join(nicPath, interfaceName)
		fileInfo, err := os.Lstat(filename)
		if err != nil {
			return interfaces, err
		}
		// If the file is a symbolic link, it is an interface.
		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink && interfaceName != "lo" {
			istats, err := getNetworkInterfaceStats(interfaceName, nicPath)
			if err != nil {
				return interfaces, err
			}
			interfaces = append(interfaces, istats)
		}
	}
	return interfaces, nil
}

// Reads the specified statistics available under /<nicPath>/<EthInterface>/statistics.
func readSysfsNetworkStats(ethInterface, statsFile, nicPath string) (uint64, error) {
	data, err := ioutil.ReadFile(filepath.Join(nicPath, ethInterface, "statistics", statsFile))
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
}

// loopback is a network strategy that provides a basic loopback device
type loopback struct {
}

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

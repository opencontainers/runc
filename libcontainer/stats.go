package libcontainer

type NetworkInterface struct {
	// Name is the name of the network interface.
	Name string `json:"name,omitempty"`

	RxBytes   uint64 `json:"rx_bytes,omitempty"`
	RxPackets uint64 `json:"rx_packets,omitempty"`
	RxErrors  uint64 `json:"rx_errors,omitempty"`
	RxDropped uint64 `json:"rx_dropped,omitempty"`
	TxBytes   uint64 `json:"tx_bytes,omitempty"`
	TxPackets uint64 `json:"tx_packets,omitempty"`
	TxErrors  uint64 `json:"tx_errors,omitempty"`
	TxDropped uint64 `json:"tx_dropped,omitempty"`
}

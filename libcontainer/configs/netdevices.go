package configs

// LinuxNetDevice represents a single network device to be added to the container's network namespace
type LinuxNetDevice struct {
	// Name of the device in the container namespace
	Name string `json:"name,omitempty"`
	// Address is the IP address and Prefix in the container namespace in CIDR fornat
	Addresses []string `json:"addresses,omitempty"`
	// HardwareAddres represents a physical hardware address.
	HardwareAddress string `json:"hardwareAddress,omitempty"`
	// MTU Maximum Transfer Unit of the network device in the container namespace
	MTU uint32 `json:"mtu,omitempty"`
}

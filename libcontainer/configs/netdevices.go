package configs

// LinuxNetDevice represents a single network device to be added to the container's network namespace.
type LinuxNetDevice struct {
	// Name of the device in the container namespace.
	Name string `json:"name,omitempty"`
}

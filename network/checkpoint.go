package network

// Struct describing the network specific checkpoint that will be maintained by libcontainer for all running containers
// This is an internal checkpoint, so do not depend on it outside of libcontainer.
type NetworkCkpt struct {
	// The name of the veth interface on the Host.
	VethHost string `json:"veth_host,omitempty"`
	// The name of the veth interface created inside the container for the child.
	VethChild string `json:"veth_child,omitempty"`
}

type NetworkCkptIntImpl struct{}

var (
	networkCkptInfo = &NetworkCkpt{}
	NetworkCkptImpl = &NetworkCkptIntImpl{}
)

func (NetworkCkptIntImpl) GetNetworkCkpt() *NetworkCkpt {
	return networkCkptInfo
}

func (NetworkCkptIntImpl) updateNetworkCkpt(n *NetworkCkpt) {
	networkCkptInfo = n
}

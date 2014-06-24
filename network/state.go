package network

// Struct describing the network specific runtime state that will be maintained by libcontainer for all running containers
// Do not depend on it outside of libcontainer.
type NetworkState struct {
	// The name of the veth interface on the Host.
	VethHost string `json:"veth_host,omitempty"`
	// The name of the veth interface created inside the container for the child.
	VethChild string `json:"veth_child,omitempty"`
}

type networkStateIntImpl struct{}

var (
	networkState     = &NetworkState{}
	NetworkStateImpl = &networkStateIntImpl{}
)

func (networkStateIntImpl) GetNetworkState() *NetworkState {
	return networkState
}

func (networkStateIntImpl) updateNetworkState(n *NetworkState) {
	networkState = n
}

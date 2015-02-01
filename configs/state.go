package configs

// State represents a running container's state
type State struct {
	// InitPid is the init process id in the parent namespace
	InitPid int `json:"init_pid,omitempty"`

	// InitStartTime is the init process start time
	InitStartTime string `json:"init_start_time,omitempty"`

	// Network runtime state.
	NetworkState NetworkState `json:"network_state,omitempty"`

	// Path to all the cgroups setup for a container. Key is cgroup subsystem name.
	CgroupPaths map[string]string `json:"cgroup_paths,omitempty"`

	Status Status `json:"status,omitempty"`
}

// Struct describing the network specific runtime state that will be maintained by libcontainer for all running containers
// Do not depend on it outside of libcontainer.
// TODO: move veth names to config time
type NetworkState struct {
	// The name of the veth interface on the Host.
	VethHost string `json:"veth_host,omitempty"`
	// The name of the veth interface created inside the container for the child.
	VethChild string `json:"veth_child,omitempty"`
}

// The status of a container.
type Status int

const (
	// The container exists and is running.
	Running Status = iota + 1

	// The container exists, it is in the process of being paused.
	Pausing

	// The container exists, but all its processes are paused.
	Paused

	// The container does not exist.
	Destroyed
)

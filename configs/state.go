package configs

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

// State represents a running container's state
type State struct {
	// InitPid is the init process id in the parent namespace
	InitPid int `json:"init_pid,omitempty"`

	// InitStartTime is the init process start time
	InitStartTime string `json:"init_start_time,omitempty"`

	// Path to all the cgroups setup for a container. Key is cgroup subsystem name.
	CgroupPaths map[string]string `json:"cgroup_paths,omitempty"`
}

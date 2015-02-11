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

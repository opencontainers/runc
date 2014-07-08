package libcontainer

type Factory interface {
	// Initializes a new container with the specified ID and config. The ID is a user-provided
	// opaque identifier. Starts an init process with the initialProcess config inside the new
	// container.
	//
	// Returns the new container and the PID of the new init inside the container.
	// The caller must reap this PID.
	//
	// Errors: ID already exists,
	//         config or initialConfig is invalid,
	//         system error.
	//
	// On error, any partially created container parts are cleaned up (the operation is atomic).
	Initialize(id string, config *Config, initialProcess *ProcessConfig) (*Container, int, error)
}

/*
NOTE: The API is in flux and mainly not implemented. Proceed with caution until further notice.
*/
package libcontainer

type ContainerInfo interface {
	// Returns the ID of the container
	ID() string

	// Returns the current run state of the container.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	RunState() (*RunState, Error)

	// Returns the current config of the container.
	Config() *Config

	// Returns the PIDs inside this container. The PIDs are in the namespace of the calling process.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	//
	// Some of the returned PIDs may no longer refer to processes in the Container, unless
	// the Container state is PAUSED in which case every PID in the slice is valid.
	Processes() ([]int, Error)

	// Returns statistics for the container.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	Stats() (*ContainerStats, Error)
}

// A libcontainer container object.
//
// Each container is thread-safe within the same process. Since a container can
// be destroyed by a separate process, any function may return that the container
// was not found.
type Container interface {
	ContainerInfo

	// Start a process inside the container. Returns the PID of the new process (in the caller process's namespace) and a channel that will return the exit status of the process whenever it dies.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// ConfigInvalid - config is invalid,
	// ContainerPaused - Container is paused,
	// SystemError - System error.
	StartProcess(config *ProcessConfig) (pid int, err Error)

	// Destroys the container after killing all running processes.
	//
	// Any event registrations are removed before the container is destroyed.
	// No error is returned if the container is already destroyed.
	//
	// Errors:
	// SystemError - System error.
	Destroy() Error

	// If the Container state is RUNNING or PAUSING, sets the Container state to PAUSING and pauses
	// the execution of any user processes. Asynchronously, when the container finished being paused the
	// state is changed to PAUSED.
	// If the Container state is PAUSED, do nothing.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	Pause() Error

	// If the Container state is PAUSED, resumes the execution of any user processes in the
	// Container before setting the Container state to RUNNING.
	// If the Container state is RUNNING, do nothing.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	Resume() Error

	// Signal sends the specified signal to a process owned by the container.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// ContainerPaused - Container is paused,
	// SystemError - System error.
	Signal(pid, signal int) Error

	// Wait waits for the init process of the conatiner to die and returns it's exit status.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	Wait() (exitStatus int, err Error)

	// WaitProcess waits on a process owned by the container.
	//
	// Errors:
	// ContainerDestroyed - Container no longer exists,
	// SystemError - System error.
	WaitProcess(pid int) (exitStatus int, err Error)
}

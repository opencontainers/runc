/*
NOTE: The API is in flux and mainly not implemented. Proceed with caution until further notice.
*/
package libcontainer

import (
	"os"

	"github.com/docker/libcontainer/configs"
)

// A libcontainer container object.
//
// Each container is thread-safe within the same process. Since a container can
// be destroyed by a separate process, any function may return that the container
// was not found.
type Container interface {
	// Returns the ID of the container
	ID() string

	// Returns the current status of the container.
	//
	// errors:
	// Systemerror - System error.
	Status() (configs.Status, error)

	// Returns the current config of the container.
	Config() configs.Config

	// Returns the PIDs inside this container. The PIDs are in the namespace of the calling process.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// Systemerror - System error.
	//
	// Some of the returned PIDs may no longer refer to processes in the Container, unless
	// the Container state is PAUSED in which case every PID in the slice is valid.
	Processes() ([]int, error)

	// Returns statistics for the container.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// Systemerror - System error.
	Stats() (*Stats, error)

	// Start a process inside the container. Returns the PID of the new process (in the caller process's namespace) and a channel that will return the exit status of the process whenever it dies.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// ConfigInvalid - config is invalid,
	// ContainerPaused - Container is paused,
	// Systemerror - System error.
	Start(process *Process) (pid int, err error)

	// Destroys the container after killing all running processes.
	//
	// Any event registrations are removed before the container is destroyed.
	// No error is returned if the container is already destroyed.
	//
	// errors:
	// Systemerror - System error.
	Destroy() error

	// If the Container state is RUNNING or PAUSING, sets the Container state to PAUSING and pauses
	// the execution of any user processes. Asynchronously, when the container finished being paused the
	// state is changed to PAUSED.
	// If the Container state is PAUSED, do nothing.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// Systemerror - System error.
	Pause() error

	// If the Container state is PAUSED, resumes the execution of any user processes in the
	// Container before setting the Container state to RUNNING.
	// If the Container state is RUNNING, do nothing.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// Systemerror - System error.
	Resume() error

	// Signal sends the specified signal to the init process of the container.
	//
	// errors:
	// ContainerDestroyed - Container no longer exists,
	// ContainerPaused - Container is paused,
	// Systemerror - System error.
	Signal(signal os.Signal) error

	// OOM returns a read-only channel signaling when the container receives an OOM notification.
	//
	// errors:
	// Systemerror - System error.
	OOM() (<-chan struct{}, error)
}

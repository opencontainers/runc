package libcontainer

import "errors"

var (
	ErrExist      = errors.New("container with given ID already exists") // ErrExist is returned if an ID is already in use by a container.
	ErrInvalidID  = errors.New("invalid container ID format")            // ErrInvalidID is returned if an ID  has incorrect format.
	ErrNotExist   = errors.New("container does not exist")               // ErrNotExist is returned if an action failed because because the container does not exist.
	ErrPaused     = errors.New("container paused")                       // ErrPaused is returned if an action failed because the container is paused.
	ErrRunning    = errors.New("container still running")                // ErrRunning is returned if an action failed because the container is still running.
	ErrNotRunning = errors.New("container not running")                  // ErrNotRunning is returned if an action failed because the container is not running.
	ErrNotPaused  = errors.New("container not paused")                   // ErrNotPaused is returned if an action failed because the container is not paused.
)

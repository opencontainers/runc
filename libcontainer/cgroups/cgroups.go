// +build linux

package cgroups

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// Manager implements cgroup manager.
type Manager interface {
	// Apply moves the process with the specified pid to all subsystem cgroups
	Apply(pid int) error

	// GetPids returns the PIDs inside the cgroup set
	GetPids() ([]int, error)

	// GetAllPids returns the PIDs inside the cgroup set & all sub-cgroups
	GetAllPids() ([]int, error)

	// GetStats returns statistics for the cgroup set
	GetStats() (*Stats, error)

	// Freeze toggles the freezer cgroup according with specified state
	Freeze(state configs.FreezerState) error

	// Destroy destroys the cgroup set
	Destroy() error

	// The option func SystemdCgroups() and Cgroupfs() require following attributes:
	// 	Paths   map[string]string
	// 	Cgroups *configs.Cgroup
	// Paths maps cgroup subsystem to path at which it is mounted.
	// Cgroups specifies specific cgroup settings for the various subsystems

	// GetPaths returns cgroup paths to save in a state file and to be able to
	// restore the object later.
	GetPaths() map[string]string

	// Set sets the cgroup as configured.
	Set(container *configs.Config) error
}

// NotFoundError implements error when subsystem is not found.
type NotFoundError struct {
	Subsystem string
}

// Error returns error message when subsystem is not found.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("mountpoint for %s not found", e.Subsystem)
}

// NewNotFoundError returns initialized NotFoundError struct.
func NewNotFoundError(sub string) error {
	return &NotFoundError{
		Subsystem: sub,
	}
}

// IsNotFound checks if an error is NotFoundError.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*NotFoundError)
	return ok
}

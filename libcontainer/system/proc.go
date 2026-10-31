package system

import (
	"github.com/opencontainers/runc/internal/sys"
)

// State is the status of a process.
//
// Deprecated: This type is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
type State = sys.State

// Process states.
//
// Deprecated: These constants are deprecated and have been moved to an
// internal package. They will be removed in runc 1.7.
const (
	Dead        = sys.Dead
	DiskSleep   = sys.DiskSleep
	Running     = sys.Running
	Sleeping    = sys.Sleeping
	Stopped     = sys.Stopped
	TracingStop = sys.TracingStop
	Zombie      = sys.Zombie
	Parked      = sys.Parked
	Idle        = sys.Idle
)

// Stat_t represents the information from /proc/[pid]/stat, as
// described in proc(5) with names based on the /proc/[pid]/status
// fields.
//
// Deprecated: This type is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
type Stat_t = sys.StatT //nolint:revive,staticcheck,nolintlint // Keep the old name. // nolintlint is needed to work around the different lint configs

// Stat returns a Stat_t instance for the specified process.
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func Stat(pid int) (Stat_t, error) {
	return sys.Stat(pid)
}

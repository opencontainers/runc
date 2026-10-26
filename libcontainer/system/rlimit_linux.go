package system

import (
	"syscall"

	"github.com/opencontainers/runc/internal/sys"
)

// ClearRlimitNofileCache clears go runtime's nofile rlimit cache. The argument
// is process RLIMIT_NOFILE values. Relies on go.dev/cl/588076.
//
// Deprecated: This function is deprecated and has been moved to an internal
// package. It will be removed in runc 1.7.
func ClearRlimitNofileCache(lim *syscall.Rlimit) {
	sys.ClearRlimitNofileCache(lim)
}

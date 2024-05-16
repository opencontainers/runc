//go:build go1.19

package system

import (
	"sync/atomic"
	"syscall"

	_ "unsafe" // for go:linkname
)

//go:linkname syscallOrigRlimitNofile syscall.origRlimitNofile
var syscallOrigRlimitNofile atomic.Pointer[syscall.Rlimit]

// ClearRlimitNofileCache is to clear go runtime's nofile rlimit cache.
func ClearRlimitNofileCache() {
	// As reported in issue #4195, the new version of go runtime(since 1.19)
	// will cache rlimit-nofile. Before executing execve, the rlimit-nofile
	// of the process will be restored with the cache. In runc, this will
	// cause the rlimit-nofile setting by the parent process for the container
	// to become invalid. It can be solved by clearing this cache. But
	// unfortunately, go stdlib doesn't provide such function, so we need to
	// link to the private var `origRlimitNofile` in package syscall to hack.
	syscallOrigRlimitNofile.Store(nil)
}

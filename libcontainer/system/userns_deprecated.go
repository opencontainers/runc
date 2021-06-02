package system

import "github.com/opencontainers/runc/libcontainer/userns"

// RunningInUserNS detects whether we are currently running in a user namespace.
// Deprecated: use github.com/opencontainers/runc/libcontainer/userns.RunningInUserNS instead
var RunningInUserNS = userns.RunningInUserNS

package fscommon

import (
	"github.com/opencontainers/runc/libcontainer/cgroups"
)

// Linux 5.14 supports the file "cgroup.kill" to kill all the processes
// in a cgroup. Check https://lwn.net/Articles/855924/
func Kill(path string) error {
	return cgroups.WriteFile(path, "cgroup.kill", "1")
}

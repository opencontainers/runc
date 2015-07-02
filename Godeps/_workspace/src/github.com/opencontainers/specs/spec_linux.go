// +build linux

package specs

// LinuxSpec is the full specification for linux containers.
type LinuxSpec struct {
	Spec
	// Linux is platform specific configuration for linux based containers.
	Linux Linux `json:"linux"`
}

// Linux contains platform specific configuration for linux based containers.
type Linux struct {
	// UidMapping specifies user mappings for supporting user namespaces on linux.
	UidMappings []IDMapping `json:"uidMappings"`
	// UidMapping specifies group mappings for supporting user namespaces on linux.
	GidMappings []IDMapping `json:"gidMappings"`
	// Rlimits specifies rlimit options to apply to the container's process.
	Rlimits []Rlimit `json:"rlimits"`
	// SystemProperties are a set of key value pairs that are set for the container on start.
	SystemProperties map[string]string `json:"systemProperties"`
	// Resources contain cgroup information for handling resource constraints
	// for the container.
	Resources Resources `json:"resources"`
	// Namespaces contains the namespaces that are created and/or joined by the container.
	Namespaces []Namespace `json:"namespaces"`
	// Capabilities are linux capabilities that are kept for the container.
	Capabilities []string `json:"capabilities"`
	// Devices are a list of device nodes that are created and enabled for the container.
	Devices []string `json:"devices"`
}

// User specifies linux specific user and group information for the container's
// main process.
type User struct {
	// Uid is the user id.
	Uid int32 `json:"uid"`
	// Gid is the group id.
	Gid int32 `json:"gid"`
	// AdditionalGids are additional group ids set the the container's process.
	AdditionalGids []int32 `json:"additionalGids"`
}

// Namespace is the configuration for a linux namespace.
type Namespace struct {
	// Type is the type of linux namespace.
	Type string `json:"type"`
	// Path is a path to an existing namespace persisted on disk that can be joined
	// and is of the same type.
	Path string `json:"path"`
}

// IDMapping specifies uid/gid mappings.
type IDMapping struct {
	// From is the uid/gid of the host user or group.
	From int32 `json:"from"`
	// To is the uid/gid of the container's user or group.
	To int32 `json:"to"`
	// Count is how many uid/gids to map after To.
	Count int32 `json:"count"`
}

// Rlimit type and restrictions.
type Rlimit struct {
	// Type of the rlimit to set.
	Type int `json:"type"`
	// Hard is the hard limit for the specified type.
	Hard uint64 `json:"hard"`
	// Soft is the soft limit for the specified type.
	Soft uint64 `json:"soft"`
}

type HugepageLimit struct {
	Pagesize string `json:"pageSize"`
	Limit    int    `json:"limit"`
}

type InterfacePriority struct {
	// Name is the name of the network interface.
	Name string `json:"name"`
	// Priority for the interface.
	Priority int64 `json:"priority"`
}

type BlockIO struct {
	// Specifies per cgroup weight, range is from 10 to 1000.
	Weight int64 `json:"blkioWeight"`
	// Weight per cgroup per device, can override BlkioWeight.
	WeightDevice string `json:"blkioWeightDevice"`
	// IO read rate limit per cgroup per device, bytes per second.
	ThrottleReadBpsDevice string `json:"blkioThrottleReadBpsDevice"`
	// IO write rate limit per cgroup per divice, bytes per second.
	ThrottleWriteBpsDevice string `json:"blkioThrottleWriteBpsDevice"`
	// IO read rate limit per cgroup per device, IO per second.
	ThrottleReadIOpsDevice string `json:"blkioThrottleReadIopsDevice"`
	// IO write rate limit per cgroup per device, IO per second.
	ThrottleWriteIOpsDevice string `json:"blkioThrottleWriteIopsDevice"`
}

type Memory struct {
	// Memory limit (in bytes)
	Limit int64 `json:"limit"`
	// Memory reservation or soft_limit (in bytes)
	Reservation int64 `json:"reservation"`
	// Total memory usage (memory + swap); set `-1' to disable swap
	Swap int64 `json:"swap"`
	// Kernel memory limit (in bytes)
	Kernel int64 `json:"kernel"`
}

type CPU struct {
	// CPU shares (relative weight vs. other cgroups with cpu shares).
	Shares int64 `json:"shares"`
	// CPU hardcap limit (in usecs). Allowed cpu time in a given period.
	Quota int64 `json:"quota"`
	// CPU period to be used for hardcapping (in usecs). 0 to use system default.
	Period int64 `json:"period"`
	// How many time CPU will use in realtime scheduling (in usecs).
	RealtimeRuntime int64 `json:"realtimeRuntime"`
	// CPU period to be used for realtime scheduling (in usecs).
	RealtimePeriod int64 `json:"realtimePeriod"`
	// CPU to use within the cpuset.
	Cpus string `json:"cpus"`
	// MEM to use within the cpuset.
	Mems string `json:"mems"`
}

type Network struct {
	// Set class identifier for container's network packets.
	ClassID string `json:"classId"`
	// Set priority of network traffic for container.
	Priorities []InterfacePriority `json:"priorities"`
}

type Resources struct {
	// DisableOOMKiller disables the OOM killer for out of memory conditions.
	DisableOOMKiller bool `json:"disableOOMKiller"`
	// Memory restriction configuration.
	Memory Memory `json:"memory"`
	// CPU resource restriction configuration.
	CPU CPU `json:"cpu"`
	// BlockIO restriction configuration.
	BlockIO BlockIO `json:"blockIO"`
	// Hugetlb limit (in bytes)
	HugepageLimits []HugepageLimit `json:"hugepageLimits"`
	// Network restriction configuration.
	Network Network `json:"network"`
}

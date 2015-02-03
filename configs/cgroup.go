package configs

type FreezerState string

const (
	Undefined FreezerState = ""
	Frozen    FreezerState = "FROZEN"
	Thawed    FreezerState = "THAWED"
)

type Cgroup struct {
	Name string `json:"name,omitempty"`

	// name of parent cgroup or slice
	Parent string `json:"parent,omitempty"`

	// If this is true allow access to any kind of device within the container.  If false, allow access only to devices explicitly listed in the allowed_devices list.
	AllowAllDevices bool `json:"allow_all_devices,omitempty"`

	AllowedDevices []*Device `json:"allowed_devices,omitempty"`

	// Memory limit (in bytes)
	Memory int64 `json:"memory,omitempty"`

	// Memory reservation or soft_limit (in bytes)
	MemoryReservation int64 `json:"memory_reservation,omitempty"`

	// Total memory usage (memory + swap); set `-1' to disable swap
	MemorySwap int64 `json:"memory_swap,omitempty"`

	// CPU shares (relative weight vs. other containers)
	CpuShares int64 `json:"cpu_shares,omitempty"`

	// CPU hardcap limit (in usecs). Allowed cpu time in a given period.
	CpuQuota int64 `json:"cpu_quota,omitempty"`

	// CPU period to be used for hardcapping (in usecs). 0 to use system default.
	CpuPeriod int64 `json:"cpu_period,omitempty"`

	// CPU to use
	CpusetCpus string `json:"cpuset_cpus,omitempty"`

	// MEM to use
	CpusetMems string `json:"cpuset_mems,omitempty"`

	// Specifies per cgroup weight, range is from 10 to 1000.
	BlkioWeight int64 `json:"blkio_weight,omitempty"`

	// set the freeze value for the process
	Freezer FreezerState `json:"freezer,omitempty"`

	// Parent slice to use for systemd TODO: remove in favor or parent
	Slice string `json:"slice,omitempty"`
}

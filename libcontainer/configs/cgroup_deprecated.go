package configs

import "github.com/opencontainers/cgroups"

// Deprecated: use [github.com/opencontainers/cgroups].
type (
	Cgroup         = cgroups.Cgroup
	Resources      = cgroups.Resources
	FreezerState   = cgroups.FreezerState
	LinuxRdma      = cgroups.LinuxRdma
	BlockIODevice  = cgroups.BlockIODevice
	WeightDevice   = cgroups.WeightDevice
	ThrottleDevice = cgroups.ThrottleDevice
	HugepageLimit  = cgroups.HugepageLimit
	IfPrioMap      = cgroups.IfPrioMap
)

// Deprecated: use [github.com/opencontainers/cgroups].
const (
	Undefined = cgroups.Undefined
	Frozen    = cgroups.Frozen
	Thawed    = cgroups.Thawed
)

// Deprecated: use [github.com/opencontainers/cgroups].
var (
	NewWeightDevice   = cgroups.NewWeightDevice
	NewThrottleDevice = cgroups.NewThrottleDevice
)

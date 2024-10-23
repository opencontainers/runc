package configs

import cg "github.com/opencontainers/runc/libcontainer/cgroups/configs"

// Deprecated: use [github.com/opencontainers/runc/libcontainer/cgroups/configs].
type (
	Cgroup         = cg.Cgroup
	Resources      = cg.Resources
	FreezerState   = cg.FreezerState
	LinuxRdma      = cg.LinuxRdma
	BlockIODevice  = cg.BlockIODevice
	WeightDevice   = cg.WeightDevice
	ThrottleDevice = cg.ThrottleDevice
	HugepageLimit  = cg.HugepageLimit
	IfPrioMap      = cg.IfPrioMap
)

// Deprecated: use [github.com/opencontainers/runc/libcontainer/cgroups/configs].
const (
	Undefined = cg.Undefined
	Frozen    = cg.Frozen
	Thawed    = cg.Thawed
)

// Deprecated: use [github.com/opencontainers/runc/libcontainer/cgroups/configs].
var (
	NewWeightDevice   = cg.NewWeightDevice
	NewThrottleDevice = cg.NewThrottleDevice
)

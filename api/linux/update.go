package linux

import (
	"context"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

func (l *Libcontainer) Update(ctx context.Context, id string, r *specs.LinuxResources, l3Cache string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	config := container.Config()
	// Update the value
	config.Cgroups.Resources.BlkioWeight = *r.BlockIO.Weight
	config.Cgroups.Resources.CpuPeriod = *r.CPU.Period
	config.Cgroups.Resources.CpuQuota = *r.CPU.Quota
	config.Cgroups.Resources.CpuShares = *r.CPU.Shares
	config.Cgroups.Resources.CpuRtPeriod = *r.CPU.RealtimePeriod
	config.Cgroups.Resources.CpuRtRuntime = *r.CPU.RealtimeRuntime
	config.Cgroups.Resources.CpusetCpus = r.CPU.Cpus
	config.Cgroups.Resources.CpusetMems = r.CPU.Mems
	config.Cgroups.Resources.KernelMemory = *r.Memory.Kernel
	config.Cgroups.Resources.KernelMemoryTCP = *r.Memory.KernelTCP
	config.Cgroups.Resources.Memory = *r.Memory.Limit
	config.Cgroups.Resources.MemoryReservation = *r.Memory.Reservation
	config.Cgroups.Resources.MemorySwap = *r.Memory.Swap
	config.Cgroups.Resources.PidsLimit = r.Pids.Limit

	// Update Intel RDT/CAT
	if l3Cache != "" {
		if !intelrdt.IsEnabled() {
			return fmt.Errorf("Intel RDT: l3 cache schema is not enabled")
		}

		// If intelRdt is not specified in original configuration, we just don't
		// Apply() to create intelRdt group or attach tasks for this container.
		// In update command, we could re-enable through IntelRdtManager.Apply()
		// and then update intelrdt constraint.
		if config.IntelRdt == nil {
			state, err := container.State()
			if err != nil {
				return err
			}
			config.IntelRdt = &configs.IntelRdt{}
			intelRdtManager := intelrdt.IntelRdtManager{
				Config: &config,
				Id:     container.ID(),
				Path:   state.IntelRdtPath,
			}
			if err := intelRdtManager.Apply(state.InitProcessPid); err != nil {
				return err
			}
		}
		config.IntelRdt.L3CacheSchema = l3Cache
	}
	return container.Set(config)
}

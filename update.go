package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"

	"github.com/opencontainers/cgroups"
	"github.com/sirupsen/logrus"

	"github.com/docker/go-units"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

func mkPtr[T any](v T) *T { return &v }

var updateCommand = cli.Command{
	Name:      "update",
	Usage:     "update container resource constraints",
	ArgsUsage: `<container-id>`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "resources, r",
			Value: "",
			Usage: `path to the file containing the resources to update or '-' to read from the standard input

The accepted format is as follow (unchanged values can be omitted):

{
  "memory": {
    "limit": 0,
    "reservation": 0,
    "swap": 0,
    "checkBeforeUpdate": true
  },
  "cpu": {
    "shares": 0,
    "quota": 0,
    "burst": 0,
    "period": 0,
    "realtimeRuntime": 0,
    "realtimePeriod": 0,
    "cpus": "",
    "mems": "",
    "idle": 0
  },
  "blockIO": {
    "weight": 0
  }
}

Note: if data is to be read from a file or the standard input, all
other options are ignored.
`,
		},

		cli.IntFlag{
			Name:  "blkio-weight",
			Usage: "Specifies per cgroup weight, range is from 10 to 1000",
		},
		cli.StringFlag{
			Name:  "cpu-period",
			Usage: "CPU CFS period to be used for hardcapping (in usecs). 0 to use system default",
		},
		cli.StringFlag{
			Name:  "cpu-quota",
			Usage: "CPU CFS hardcap limit (in usecs). Allowed cpu time in a given period",
		},
		cli.StringFlag{
			Name:  "cpu-burst",
			Usage: "CPU CFS hardcap burst limit (in usecs). Allowed accumulated cpu time additionally for burst a given period",
		},
		cli.StringFlag{
			Name:  "cpu-share",
			Usage: "CPU shares (relative weight vs. other containers)",
		},
		cli.StringFlag{
			Name:  "cpu-rt-period",
			Usage: "CPU realtime period to be used for hardcapping (in usecs). 0 to use system default",
		},
		cli.StringFlag{
			Name:  "cpu-rt-runtime",
			Usage: "CPU realtime hardcap limit (in usecs). Allowed cpu time in a given period",
		},
		cli.StringFlag{
			Name:  "cpuset-cpus",
			Usage: "CPU(s) to use",
		},
		cli.StringFlag{
			Name:  "cpuset-mems",
			Usage: "Memory node(s) to use",
		},
		cli.StringFlag{
			Name:   "kernel-memory",
			Usage:  "(obsoleted; do not use)",
			Hidden: true,
		},
		cli.StringFlag{
			Name:   "kernel-memory-tcp",
			Usage:  "(obsoleted; do not use)",
			Hidden: true,
		},
		cli.StringFlag{
			Name:  "memory",
			Usage: "Memory limit (in bytes)",
		},
		cli.StringFlag{
			Name:  "cpu-idle",
			Usage: "set cgroup SCHED_IDLE or not, 0: default behavior, 1: SCHED_IDLE",
		},
		cli.StringFlag{
			Name:  "memory-reservation",
			Usage: "Memory reservation or soft_limit (in bytes)",
		},
		cli.StringFlag{
			Name:  "memory-swap",
			Usage: "Total memory usage (memory + swap); set '-1' to enable unlimited swap",
		},
		cli.IntFlag{
			Name:  "pids-limit",
			Usage: "Maximum number of pids allowed in the container",
		},
		cli.StringFlag{
			Name:  "l3-cache-schema",
			Usage: "The string of Intel RDT/CAT L3 cache schema",
		},
		cli.StringFlag{
			Name:  "mem-bw-schema",
			Usage: "The string of Intel RDT/MBA memory bandwidth schema",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}

		r := specs.LinuxResources{
			// nil and mkPtr(0) are not interchangeable
			Memory: &specs.LinuxMemory{
				CheckBeforeUpdate: mkPtr(false), // constant
			},
			CPU:     &specs.LinuxCPU{},
			BlockIO: &specs.LinuxBlockIO{},
			Pids:    &specs.LinuxPids{},
		}

		config := container.Config()

		if in := context.String("resources"); in != "" {
			var (
				f   *os.File
				err error
			)
			switch in {
			case "-":
				f = os.Stdin
			default:
				f, err = os.Open(in)
				if err != nil {
					return err
				}
				defer f.Close()
			}
			err = json.NewDecoder(f).Decode(&r)
			if err != nil {
				return err
			}
		} else {
			if val := context.Int("blkio-weight"); val != 0 {
				r.BlockIO.Weight = mkPtr(uint16(val))
			}
			if val := context.String("cpuset-cpus"); val != "" {
				r.CPU.Cpus = val
			}
			if val := context.String("cpuset-mems"); val != "" {
				r.CPU.Mems = val
			}
			if val := context.String("cpu-idle"); val != "" {
				idle, err := strconv.ParseInt(val, 10, 64)
				if err != nil {
					return fmt.Errorf("invalid value for cpu-idle: %w", err)
				}
				r.CPU.Idle = mkPtr(idle)
			}

			for _, pair := range []struct {
				opt  string
				dest **uint64
			}{
				{"cpu-burst", &r.CPU.Burst},
				{"cpu-period", &r.CPU.Period},
				{"cpu-rt-period", &r.CPU.RealtimePeriod},
				{"cpu-share", &r.CPU.Shares},
			} {
				if val := context.String(pair.opt); val != "" {
					v, err := strconv.ParseUint(val, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %w", pair.opt, err)
					}
					*pair.dest = &v
				}
			}
			for _, pair := range []struct {
				opt  string
				dest **int64
			}{
				{"cpu-quota", &r.CPU.Quota},
				{"cpu-rt-runtime", &r.CPU.RealtimeRuntime},
			} {
				if val := context.String(pair.opt); val != "" {
					v, err := strconv.ParseInt(val, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %w", pair.opt, err)
					}
					*pair.dest = &v
				}
			}
			for _, pair := range []struct {
				opt  string
				dest **int64
			}{
				{"memory", &r.Memory.Limit},
				{"memory-swap", &r.Memory.Swap},
				{"kernel-memory", &r.Memory.Kernel}, //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
				{"kernel-memory-tcp", &r.Memory.KernelTCP},
				{"memory-reservation", &r.Memory.Reservation},
			} {
				if val := context.String(pair.opt); val != "" {
					var v int64

					if val != "-1" {
						v, err = units.RAMInBytes(val)
						if err != nil {
							return fmt.Errorf("invalid value for %s: %w", pair.opt, err)
						}
					} else {
						v = -1
					}
					*pair.dest = &v
				}
			}

			if context.IsSet("pids-limit") {
				r.Pids.Limit = mkPtr(int64(context.Int("pids-limit")))
			}
		}

		// Fix up values
		if r.Memory.Limit != nil && *r.Memory.Limit == -1 && r.Memory.Swap == nil {
			// To avoid error "unable to set swap limit without memory limit"
			r.Memory.Swap = mkPtr[int64](0)
		}
		if r.CPU.Idle != nil && r.CPU.Shares == nil {
			// To avoid error "failed to write \"4\": write /sys/fs/cgroup/runc-cgroups-integration-test/test-cgroup-7341/cpu.weight: invalid argument"
			r.CPU.Shares = mkPtr[uint64](0)
		}

		if (r.Memory.Kernel != nil) || (r.Memory.KernelTCP != nil) { //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
			logrus.Warn("Kernel memory settings are ignored and will be removed")
		}

		// Update the values
		if r.BlockIO.Weight != nil {
			config.Cgroups.Resources.BlkioWeight = *r.BlockIO.Weight
		}
		if r.BlockIO.LeafWeight != nil {
			config.Cgroups.Resources.BlkioLeafWeight = *r.BlockIO.LeafWeight
		}
		// For devices, we either update an existing one, or insert a new one.
		for _, wd := range r.BlockIO.WeightDevice {
			config.Cgroups.Resources.BlkioWeightDevice = upsertWeightDevice(config.Cgroups.Resources.BlkioWeightDevice, wd)
		}
		for _, td := range r.BlockIO.ThrottleReadBpsDevice {
			config.Cgroups.Resources.BlkioThrottleReadBpsDevice = upsertThrottleDevice(config.Cgroups.Resources.BlkioThrottleReadBpsDevice, td)
		}
		for _, td := range r.BlockIO.ThrottleWriteBpsDevice {
			config.Cgroups.Resources.BlkioThrottleWriteBpsDevice = upsertThrottleDevice(config.Cgroups.Resources.BlkioThrottleWriteBpsDevice, td)
		}
		for _, td := range r.BlockIO.ThrottleReadIOPSDevice {
			config.Cgroups.Resources.BlkioThrottleReadIOPSDevice = upsertThrottleDevice(config.Cgroups.Resources.BlkioThrottleReadIOPSDevice, td)
		}
		for _, td := range r.BlockIO.ThrottleWriteIOPSDevice {
			config.Cgroups.Resources.BlkioThrottleWriteIOPSDevice = upsertThrottleDevice(config.Cgroups.Resources.BlkioThrottleWriteIOPSDevice, td)
		}

		// Setting CPU quota and period independently does not make much sense,
		// but historically runc allowed it and this needs to be supported
		// to not break compatibility.
		//
		// For systemd cgroup drivers to set CPU quota/period correctly,
		// it needs to know both values. For fs2 cgroup driver to be compatible
		// with the fs driver, it also needs to know both values.
		//
		// Here in update, previously set values are available from config.
		// If only one of {quota,period} is set and the other is not, leave
		// the unset parameter at the old value (don't overwrite config).
		var (
			p uint64
			q int64
		)
		if r.CPU.Period != nil {
			p = *r.CPU.Period
		}
		if r.CPU.Quota != nil {
			q = *r.CPU.Quota
		}
		if (p == 0 && q == 0) || (p != 0 && q != 0) {
			// both values are either set or unset (0)
			config.Cgroups.Resources.CpuPeriod = p
			config.Cgroups.Resources.CpuQuota = q
		} else {
			// one is set and the other is not
			if p != 0 {
				// set new period, leave quota at old value
				config.Cgroups.Resources.CpuPeriod = p
			} else if q != 0 {
				// set new quota, leave period at old value
				config.Cgroups.Resources.CpuQuota = q
			}
		}

		config.Cgroups.Resources.CpuBurst = r.CPU.Burst // can be nil
		if r.CPU.Shares != nil {
			config.Cgroups.Resources.CpuShares = *r.CPU.Shares
			// CpuWeight is used for cgroupv2 and should be converted
			config.Cgroups.Resources.CpuWeight = cgroups.ConvertCPUSharesToCgroupV2Value(*r.CPU.Shares)
		}
		if r.CPU.RealtimePeriod != nil {
			config.Cgroups.Resources.CpuRtPeriod = *r.CPU.RealtimePeriod
		}
		if r.CPU.RealtimeRuntime != nil {
			config.Cgroups.Resources.CpuRtRuntime = *r.CPU.RealtimeRuntime
		}
		config.Cgroups.Resources.CpusetCpus = r.CPU.Cpus
		config.Cgroups.Resources.CpusetMems = r.CPU.Mems
		if r.Memory.Limit != nil {
			config.Cgroups.Resources.Memory = *r.Memory.Limit
		}
		config.Cgroups.Resources.CPUIdle = r.CPU.Idle
		if r.Memory.Reservation != nil {
			config.Cgroups.Resources.MemoryReservation = *r.Memory.Reservation
		}
		if r.Memory.Swap != nil {
			config.Cgroups.Resources.MemorySwap = *r.Memory.Swap
		}
		if r.Memory.CheckBeforeUpdate != nil {
			config.Cgroups.Resources.MemoryCheckBeforeUpdate = *r.Memory.CheckBeforeUpdate
		}
		config.Cgroups.Resources.PidsLimit = r.Pids.Limit
		config.Cgroups.Resources.Unified = r.Unified

		// Update Intel RDT
		l3CacheSchema := context.String("l3-cache-schema")
		memBwSchema := context.String("mem-bw-schema")
		if l3CacheSchema != "" && !intelrdt.IsCATEnabled() {
			return errors.New("Intel RDT/CAT: l3 cache schema is not enabled")
		}

		if memBwSchema != "" && !intelrdt.IsMBAEnabled() {
			return errors.New("Intel RDT/MBA: memory bandwidth schema is not enabled")
		}

		if l3CacheSchema != "" || memBwSchema != "" {
			// If intelRdt is not specified in original configuration, we just don't
			// Apply() to create intelRdt group or attach tasks for this container.
			// In update command, we could re-enable through IntelRdtManager.Apply()
			// and then update intelrdt constraint.
			if config.IntelRdt == nil {
				return errors.New("updating a non-existent Intel RDT configuration is not supported")
			}
			if l3CacheSchema != "" {
				config.IntelRdt.L3CacheSchema = l3CacheSchema
			}
			if memBwSchema != "" {
				config.IntelRdt.MemBwSchema = memBwSchema
			}
		}

		// XXX(kolyshkin@): currently "runc update" is unable to change
		// device configuration, so add this to skip device update.
		// This helps in case an extra plugin (nvidia GPU) applies some
		// configuration on top of what runc does.
		// Note this field is not saved into container's state.json.
		config.Cgroups.SkipDevices = true

		return container.Set(config)
	},
}

func upsertWeightDevice(devices []*cgroups.WeightDevice, wd specs.LinuxWeightDevice) []*cgroups.WeightDevice {
	// Iterate backwards because in case of a duplicate
	// the last one will be used.
	for i, dev := range slices.Backward(devices) {
		if dev.Major != wd.Major || dev.Minor != wd.Minor {
			continue
		}
		// Update weights for existing device.
		if wd.Weight != nil {
			devices[i].Weight = *wd.Weight
		}
		if wd.LeafWeight != nil {
			devices[i].LeafWeight = *wd.LeafWeight
		}
		return devices
	}

	// New device -- append it.
	var weight, leafWeight uint16
	if wd.Weight != nil {
		weight = *wd.Weight
	}
	if wd.LeafWeight != nil {
		leafWeight = *wd.LeafWeight
	}

	return append(devices, cgroups.NewWeightDevice(wd.Major, wd.Minor, weight, leafWeight))
}

func upsertThrottleDevice(devices []*cgroups.ThrottleDevice, td specs.LinuxThrottleDevice) []*cgroups.ThrottleDevice {
	// Iterate backwards because in case of a duplicate
	// the last one will be used.
	for i, dev := range slices.Backward(devices) {
		if dev.Major == td.Major && dev.Minor == td.Minor {
			devices[i].Rate = td.Rate
			return devices
		}
	}

	return append(devices, cgroups.NewThrottleDevice(td.Major, td.Minor, td.Rate))
}

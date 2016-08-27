// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"

	"github.com/docker/go-units"
	"github.com/opencontainers/runc/libcontainer/configs"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

func u64Ptr(i uint64) *uint64 { return &i }
func u16Ptr(i uint16) *uint16 { return &i }

var regBlkioWeightDevice = regexp.MustCompile(`([0-9]+):([0-9]+) ([0-9]+)(?: ([0-9]+))?`)
var regBlkioThrottleDevice = regexp.MustCompile(`([0-9]+):([0-9]+) ([0-9]+)`)

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
    "kernel": 0,
    "kernelTCP": 0
  },
  "cpu": {
    "shares": 0,
    "quota": 0,
    "period": 0,
    "cpus": "",
    "mems": ""
  },
  "blockIO": {
    "blkioWeight": 0,
    "blkioLeafWeight": 0,
    "blkioWeightDevice": "",
    "blkioThrottleReadBpsDevice": "",
    "blkioThrottleWriteBpsDevice": "",
    "blkioThrottleReadIOPSDevice": "",
    "blkioThrottleWriteIOPSDevice": ""
  },
}

Note: if data is to be read from a file or the standard input, all
other options are ignored.
`,
		},

		cli.IntFlag{
			Name:  "blkio-weight",
			Usage: "Specifies per cgroup weight",
		},
		cli.IntFlag{
			Name:  "blkio-leaf-weight",
			Usage: "Specifies tasks' weight in the given cgroup while competing with the cgroup's child cgroups, cfq scheduler only",
		},
		cli.StringSliceFlag{
			Name:  "blkio-weight-device",
			Usage: "Weight per cgroup per device, can override blkio-weight. Argument must be of the form \"<MAJOR>:<MINOR> <WEIGHT> [LEAF_WEIGHT]\"",
		},
		cli.StringSliceFlag{
			Name:  "blkio-throttle-readbps-device",
			Usage: "IO read rate limit per cgroup per device, bytes per second. Argument must be of the form \"<MAJOR>:<MINOR> <RATE>\"",
		},
		cli.StringSliceFlag{
			Name:  "blkio-throttle-writebps-device",
			Usage: "IO write rate limit per cgroup per divice, bytes per second. Argument must be of the form \"<MAJOR>:<MINOR> <RATE>\"",
		},
		cli.StringSliceFlag{
			Name:  "blkio-throttle-readiops-device",
			Usage: "IO read rate limit per cgroup per device, IO per second. Argument must be of the form \"<MAJOR>:<MINOR> <RATE>\"",
		},
		cli.StringSliceFlag{
			Name:  "blkio-throttle-writeiops-device",
			Usage: "IO write rate limit per cgroup per device, IO per second. Argument must be of the form \"<MAJOR>:<MINOR> <RATE>\"",
		},
		cli.StringFlag{
			Name:  "cpu-period",
			Usage: "CPU period to be used for hardcapping (in usecs). 0 to use system default",
		},
		cli.StringFlag{
			Name:  "cpu-quota",
			Usage: "CPU hardcap limit (in usecs). Allowed cpu time in a given period",
		},
		cli.StringFlag{
			Name:  "cpu-share",
			Usage: "CPU shares (relative weight vs. other containers)",
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
			Name:  "kernel-memory",
			Usage: "Kernel memory limit (in bytes)",
		},
		cli.StringFlag{
			Name:  "kernel-memory-tcp",
			Usage: "Kernel memory limit (in bytes) for tcp buffer",
		},
		cli.StringFlag{
			Name:  "memory",
			Usage: "Memory limit (in bytes)",
		},
		cli.StringFlag{
			Name:  "memory-reservation",
			Usage: "Memory reservation or soft_limit (in bytes)",
		},
		cli.StringFlag{
			Name:  "memory-swap",
			Usage: "Total memory usage (memory + swap); set '-1' to enable unlimited swap",
		},
	},
	Action: func(context *cli.Context) error {
		container, err := getContainer(context)
		if err != nil {
			return err
		}

		r := specs.Resources{
			Memory: &specs.Memory{
				Limit:       u64Ptr(0),
				Reservation: u64Ptr(0),
				Swap:        u64Ptr(0),
				Kernel:      u64Ptr(0),
				KernelTCP:   u64Ptr(0),
			},
			CPU: &specs.CPU{
				Shares: u64Ptr(0),
				Quota:  u64Ptr(0),
				Period: u64Ptr(0),
				Cpus:   sPtr(""),
				Mems:   sPtr(""),
			},
			BlockIO: &specs.BlockIO{
				Weight:     u16Ptr(0),
				LeafWeight: u16Ptr(0),
			},
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
			}
			err = json.NewDecoder(f).Decode(&r)
			if err != nil {
				return err
			}
		} else {
			if val := context.Int("blkio-weight"); val != 0 {
				r.BlockIO.Weight = u16Ptr(uint16(val))
			}
			if val := context.Int("blkio-leaf-weight"); val != 0 {
				r.BlockIO.LeafWeight = u16Ptr(uint16(val))
			}
			if val := context.StringSlice("blkio-weight-device"); val != nil {
				if r.BlockIO.WeightDevice, err = getBlkioWeightDeviceFromString(val); err != nil {
					return fmt.Errorf("invalid value for blkio-weight-device: %v", err)
				}
			}
			for opt, dest := range map[string]*[]specs.ThrottleDevice{
				"blkio-throttle-readbps-device":   &r.BlockIO.ThrottleReadBpsDevice,
				"blkio-throttle-writebps-device":  &r.BlockIO.ThrottleWriteBpsDevice,
				"blkio-throttle-readiops-device":  &r.BlockIO.ThrottleReadIOPSDevice,
				"blkio-throttle-writeiops-device": &r.BlockIO.ThrottleWriteIOPSDevice,
			} {
				if val := context.StringSlice(opt); val != nil {
					if *dest, err = getBlkioThrottleDeviceFromString(val); err != nil {
						return fmt.Errorf("invalid value for %s: %v", opt, err)
					}
				}
			}
			if val := context.String("cpuset-cpus"); val != "" {
				r.CPU.Cpus = &val
			}
			if val := context.String("cpuset-mems"); val != "" {
				r.CPU.Mems = &val
			}

			for opt, dest := range map[string]*uint64{
				"cpu-period": r.CPU.Period,
				"cpu-quota":  r.CPU.Quota,
				"cpu-share":  r.CPU.Shares,
			} {
				if val := context.String(opt); val != "" {
					var err error
					*dest, err = strconv.ParseUint(val, 10, 64)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %s", opt, err)
					}
				}
			}

			for opt, dest := range map[string]*uint64{
				"kernel-memory":      r.Memory.Kernel,
				"kernel-memory-tcp":  r.Memory.KernelTCP,
				"memory":             r.Memory.Limit,
				"memory-reservation": r.Memory.Reservation,
				"memory-swap":        r.Memory.Swap,
			} {
				if val := context.String(opt); val != "" {
					v, err := units.RAMInBytes(val)
					if err != nil {
						return fmt.Errorf("invalid value for %s: %s", opt, err)
					}
					*dest = uint64(v)
				}
			}
		}

		// Update the value
		config.Cgroups.Resources.BlkioWeight = *r.BlockIO.Weight
		config.Cgroups.Resources.BlkioLeafWeight = *r.BlockIO.LeafWeight
		config.Cgroups.Resources.BlkioWeightDevice = convertSpecWeightDevices(r.BlockIO.WeightDevice)
		config.Cgroups.Resources.BlkioThrottleReadBpsDevice = convertSpecThrottleDevices(r.BlockIO.ThrottleReadBpsDevice)
		config.Cgroups.Resources.BlkioThrottleWriteBpsDevice = convertSpecThrottleDevices(r.BlockIO.ThrottleWriteBpsDevice)
		config.Cgroups.Resources.BlkioThrottleReadIOPSDevice = convertSpecThrottleDevices(r.BlockIO.ThrottleReadIOPSDevice)
		config.Cgroups.Resources.BlkioThrottleWriteIOPSDevice = convertSpecThrottleDevices(r.BlockIO.ThrottleWriteIOPSDevice)
		config.Cgroups.Resources.CpuPeriod = int64(*r.CPU.Period)
		config.Cgroups.Resources.CpuQuota = int64(*r.CPU.Quota)
		config.Cgroups.Resources.CpuShares = int64(*r.CPU.Shares)
		config.Cgroups.Resources.CpusetCpus = *r.CPU.Cpus
		config.Cgroups.Resources.CpusetMems = *r.CPU.Mems
		config.Cgroups.Resources.KernelMemory = int64(*r.Memory.Kernel)
		config.Cgroups.Resources.KernelMemoryTCP = int64(*r.Memory.KernelTCP)
		config.Cgroups.Resources.Memory = int64(*r.Memory.Limit)
		config.Cgroups.Resources.MemoryReservation = int64(*r.Memory.Reservation)
		config.Cgroups.Resources.MemorySwap = int64(*r.Memory.Swap)

		if err := container.Set(config); err != nil {
			return err
		}
		return nil
	},
}

func getBlkioWeightDeviceFromString(deviceStr []string) ([]specs.WeightDevice, error) {
	var devs []specs.WeightDevice
	for _, s := range deviceStr {
		elems := regBlkioWeightDevice.FindStringSubmatch(s)
		if elems == nil || len(elems) < 5 {
			return nil, fmt.Errorf("invalid value for %s", s)
		}
		var dev specs.WeightDevice
		var err error
		var weight uint64
		if dev.Major, err = strconv.ParseInt(elems[1], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[1], err)
		}
		if dev.Minor, err = strconv.ParseInt(elems[2], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[2], err)
		}
		if weight, err = strconv.ParseUint(elems[3], 10, 16); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[3], err)
		}
		dev.Weight = u16Ptr(uint16(weight))
		if elems[4] != "" {
			if weight, err = strconv.ParseUint(elems[4], 10, 16); err != nil {
				return nil, fmt.Errorf("invalid value for %s, err: %v", elems[4], err)
			}
			dev.LeafWeight = u16Ptr(uint16(weight))
		}
		devs = append(devs, dev)
	}

	return devs, nil
}

func getBlkioThrottleDeviceFromString(deviceStr []string) ([]specs.ThrottleDevice, error) {
	var devs []specs.ThrottleDevice
	for _, s := range deviceStr {
		elems := regBlkioThrottleDevice.FindStringSubmatch(s)
		if elems == nil || len(elems) < 4 {
			return nil, fmt.Errorf("invalid value for %s", s)
		}
		var dev specs.ThrottleDevice
		var err error
		var rate uint64
		if dev.Major, err = strconv.ParseInt(elems[1], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[1], err)
		}
		if dev.Minor, err = strconv.ParseInt(elems[2], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[2], err)
		}
		if rate, err = strconv.ParseUint(elems[3], 10, 64); err != nil {
			return nil, fmt.Errorf("invalid value for %s, err: %v", elems[3], err)
		}
		dev.Rate = u64Ptr(rate)
		devs = append(devs, dev)
	}
	return devs, nil
}

func convertSpecWeightDevices(devices []specs.WeightDevice) []*configs.WeightDevice {
	var cwds []*configs.WeightDevice
	for _, d := range devices {
		leafWeight := uint16(0)
		if d.LeafWeight != nil {
			leafWeight = *d.LeafWeight
		}
		wd := configs.NewWeightDevice(d.Major, d.Minor, *d.Weight, leafWeight)
		cwds = append(cwds, wd)
	}
	return cwds
}

func convertSpecThrottleDevices(devices []specs.ThrottleDevice) []*configs.ThrottleDevice {
	var ctds []*configs.ThrottleDevice
	for _, d := range devices {
		td := configs.NewThrottleDevice(d.Major, d.Minor, *d.Rate)
		ctds = append(ctds, td)
	}
	return ctds
}

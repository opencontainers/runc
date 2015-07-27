// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/opencontainers/specs"
)

var specCommand = cli.Command{
	Name:  "spec",
	Usage: "create a new specification file",
	Action: func(context *cli.Context) {
		spec := specs.LinuxSpec{
			Spec: specs.Spec{
				Version: specs.Version,
				Platform: specs.Platform{
					OS:   runtime.GOOS,
					Arch: runtime.GOARCH,
				},
				Root: specs.Root{
					Path:     "rootfs",
					Readonly: true,
				},
				Process: specs.Process{
					Terminal: true,
					User:     specs.User{},
					Args: []string{
						"sh",
					},
					Env: []string{
						"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
						"TERM=xterm",
					},
				},
				Hostname: "shell",
				Mounts: []specs.Mount{
					{
						Type:        "proc",
						Source:      "proc",
						Destination: "/proc",
						Options:     "",
					},
					{
						Type:        "tmpfs",
						Source:      "tmpfs",
						Destination: "/dev",
						Options:     "nosuid,strictatime,mode=755,size=65536k",
					},
					{
						Type:        "devpts",
						Source:      "devpts",
						Destination: "/dev/pts",
						Options:     "nosuid,noexec,newinstance,ptmxmode=0666,mode=0620,gid=5",
					},
					{
						Type:        "tmpfs",
						Source:      "shm",
						Destination: "/dev/shm",
						Options:     "nosuid,noexec,nodev,mode=1777,size=65536k",
					},
					{
						Type:        "mqueue",
						Source:      "mqueue",
						Destination: "/dev/mqueue",
						Options:     "nosuid,noexec,nodev",
					},
					{
						Type:        "sysfs",
						Source:      "sysfs",
						Destination: "/sys",
						Options:     "nosuid,noexec,nodev",
					},
					{
						Type:        "cgroup",
						Source:      "cgroup",
						Destination: "/sys/fs/cgroup",
						Options:     "nosuid,noexec,nodev,relatime,ro",
					},
				},
			},
			Linux: specs.Linux{
				Namespaces: []specs.Namespace{
					{
						Type: "process",
					},
					{
						Type: "network",
					},
					{
						Type: "ipc",
					},
					{
						Type: "uts",
					},
					{
						Type: "mount",
					},
				},
				Capabilities: []string{
					"AUDIT_WRITE",
					"KILL",
					"NET_BIND_SERVICE",
				},
				Devices: []string{
					"null",
					"random",
					"full",
					"tty",
					"zero",
					"urandom",
				},
				Resources: specs.Resources{
					Memory: specs.Memory{
						Swappiness: -1,
					},
				},
			},
		}
		data, err := json.MarshalIndent(&spec, "", "\t")
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("%s", data)
	},
}

var namespaceMapping = map[string]configs.NamespaceType{
	"process": configs.NEWPID,
	"network": configs.NEWNET,
	"mount":   configs.NEWNS,
	"user":    configs.NEWUSER,
	"ipc":     configs.NEWIPC,
	"uts":     configs.NEWUTS,
}

// loadSpec loads the specification from the provided path.
// If the path is empty then the default path will be "config.json"
func loadSpec(path string) (*specs.LinuxSpec, error) {
	if path == "" {
		path = "config.json"
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file for %s not found", path)
		}
		return nil, err
	}
	defer f.Close()
	var s *specs.LinuxSpec
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return s, checkSpecVersion(s)
}

// checkSpecVersion makes sure that the spec version matches runc's while we are in the initial
// development period.  It is better to hard fail than have missing fields or options in the spec.
func checkSpecVersion(s *specs.LinuxSpec) error {
	if s.Version != specs.Version {
		return fmt.Errorf("spec version is not compatible with implemented version %q: spec %q", specs.Version, s.Version)
	}
	return nil
}

func createLibcontainerConfig(spec *specs.LinuxSpec) (*configs.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	rootfsPath := spec.Root.Path
	if !filepath.IsAbs(rootfsPath) {
		rootfsPath = filepath.Join(cwd, rootfsPath)
	}
	config := &configs.Config{
		Rootfs:       rootfsPath,
		Capabilities: spec.Linux.Capabilities,
		Readonlyfs:   spec.Root.Readonly,
		Hostname:     spec.Hostname,
		Privatefs:    true,
	}
	for _, ns := range spec.Linux.Namespaces {
		t, exists := namespaceMapping[ns.Type]
		if !exists {
			return nil, fmt.Errorf("namespace %q does not exist", ns)
		}
		config.Namespaces.Add(t, ns.Path)
	}
	if config.Namespaces.Contains(configs.NEWNET) {
		config.Networks = []*configs.Network{
			{
				Type: "loopback",
			},
		}
	}
	for _, m := range spec.Mounts {
		config.Mounts = append(config.Mounts, createLibcontainerMount(cwd, m))
	}
	if err := createDevices(spec, config); err != nil {
		return nil, err
	}
	if err := setupUserNamespace(spec, config); err != nil {
		return nil, err
	}
	c, err := createCgroupConfig(spec, config.Devices)
	if err != nil {
		return nil, err
	}
	config.Cgroups = c
	if config.Readonlyfs {
		setReadonly(config)
		config.MaskPaths = []string{
			"/proc/kcore",
		}
		config.ReadonlyPaths = []string{
			"/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus",
		}
	}
	config.Sysctl = spec.Linux.Sysctl
	return config, nil
}

func createLibcontainerMount(cwd string, m specs.Mount) *configs.Mount {
	flags, data := parseMountOptions(m.Options)
	source := m.Source
	if m.Type == "bind" {
		if !filepath.IsAbs(source) {
			source = filepath.Join(cwd, m.Source)
		}
	}
	return &configs.Mount{
		Device:      m.Type,
		Source:      source,
		Destination: m.Destination,
		Data:        data,
		Flags:       flags,
	}
}

func createCgroupConfig(spec *specs.LinuxSpec, devices []*configs.Device) (*configs.Cgroup, error) {
	myCgroupPath, err := cgroups.GetThisCgroupDir("devices")
	if err != nil {
		return nil, err
	}
	c := &configs.Cgroup{
		Name:           getDefaultID(),
		Parent:         myCgroupPath,
		AllowedDevices: append(devices, allowedDevices...),
	}
	r := spec.Linux.Resources
	c.Memory = r.Memory.Limit
	c.MemoryReservation = r.Memory.Reservation
	c.MemorySwap = r.Memory.Swap
	c.KernelMemory = r.Memory.Kernel
	c.MemorySwappiness = r.Memory.Swappiness
	c.CpuShares = r.CPU.Shares
	c.CpuQuota = r.CPU.Quota
	c.CpuPeriod = r.CPU.Period
	c.CpuRtRuntime = r.CPU.RealtimeRuntime
	c.CpuRtPeriod = r.CPU.RealtimePeriod
	c.CpusetCpus = r.CPU.Cpus
	c.CpusetMems = r.CPU.Mems
	c.BlkioThrottleReadBpsDevice = r.BlockIO.ThrottleReadBpsDevice
	c.BlkioThrottleWriteBpsDevice = r.BlockIO.ThrottleWriteBpsDevice
	c.BlkioThrottleReadIOpsDevice = r.BlockIO.ThrottleReadIOpsDevice
	c.BlkioThrottleWriteIOpsDevice = r.BlockIO.ThrottleWriteIOpsDevice
	c.BlkioWeight = r.BlockIO.Weight
	c.BlkioWeightDevice = r.BlockIO.WeightDevice
	for _, l := range r.HugepageLimits {
		c.HugetlbLimit = append(c.HugetlbLimit, &configs.HugepageLimit{
			Pagesize: l.Pagesize,
			Limit:    l.Limit,
		})
	}
	c.OomKillDisable = r.DisableOOMKiller
	c.NetClsClassid = r.Network.ClassID
	for _, m := range r.Network.Priorities {
		c.NetPrioIfpriomap = append(c.NetPrioIfpriomap, &configs.IfPrioMap{
			Interface: m.Name,
			Priority:  m.Priority,
		})
	}
	return c, nil
}

func createDevices(spec *specs.LinuxSpec, config *configs.Config) error {
	for _, name := range spec.Linux.Devices {
		d, err := devices.DeviceFromPath(filepath.Join("/dev", name), "rwm")
		if err != nil {
			return err
		}
		config.Devices = append(config.Devices, d)
	}
	return nil
}

func setReadonly(config *configs.Config) {
	for _, m := range config.Mounts {
		if m.Device == "sysfs" {
			m.Flags |= syscall.MS_RDONLY
		}
	}
}

func setupUserNamespace(spec *specs.LinuxSpec, config *configs.Config) error {
	if len(spec.Linux.UidMappings) == 0 {
		return nil
	}
	config.Namespaces.Add(configs.NEWUSER, "")
	create := func(m specs.IDMapping) configs.IDMap {
		return configs.IDMap{
			HostID:      int(m.HostID),
			ContainerID: int(m.ContainerID),
			Size:        int(m.Size),
		}
	}
	for _, m := range spec.Linux.UidMappings {
		config.UidMappings = append(config.UidMappings, create(m))
	}
	for _, m := range spec.Linux.GidMappings {
		config.GidMappings = append(config.GidMappings, create(m))
	}
	rootUid, err := config.HostUID()
	if err != nil {
		return err
	}
	rootGid, err := config.HostGID()
	if err != nil {
		return err
	}
	for _, node := range config.Devices {
		node.Uid = uint32(rootUid)
		node.Gid = uint32(rootGid)
	}
	return nil
}

// parseMountOptions parses the string and returns the flags and any mount data that
// it contains.
func parseMountOptions(options string) (int, string) {
	var (
		flag int
		data []string
	)
	flags := map[string]struct {
		clear bool
		flag  int
	}{
		"async":         {true, syscall.MS_SYNCHRONOUS},
		"atime":         {true, syscall.MS_NOATIME},
		"bind":          {false, syscall.MS_BIND},
		"defaults":      {false, 0},
		"dev":           {true, syscall.MS_NODEV},
		"diratime":      {true, syscall.MS_NODIRATIME},
		"dirsync":       {false, syscall.MS_DIRSYNC},
		"exec":          {true, syscall.MS_NOEXEC},
		"mand":          {false, syscall.MS_MANDLOCK},
		"noatime":       {false, syscall.MS_NOATIME},
		"nodev":         {false, syscall.MS_NODEV},
		"nodiratime":    {false, syscall.MS_NODIRATIME},
		"noexec":        {false, syscall.MS_NOEXEC},
		"nomand":        {true, syscall.MS_MANDLOCK},
		"norelatime":    {true, syscall.MS_RELATIME},
		"nostrictatime": {true, syscall.MS_STRICTATIME},
		"nosuid":        {false, syscall.MS_NOSUID},
		"private":       {false, syscall.MS_PRIVATE},
		"rbind":         {false, syscall.MS_BIND | syscall.MS_REC},
		"relatime":      {false, syscall.MS_RELATIME},
		"remount":       {false, syscall.MS_REMOUNT},
		"ro":            {false, syscall.MS_RDONLY},
		"rprivate":      {false, syscall.MS_PRIVATE | syscall.MS_REC},
		"rshared":       {false, syscall.MS_SHARED | syscall.MS_REC},
		"rslave":        {false, syscall.MS_SLAVE | syscall.MS_REC},
		"runbindable":   {false, syscall.MS_UNBINDABLE | syscall.MS_REC},
		"rw":            {true, syscall.MS_RDONLY},
		"shared":        {false, syscall.MS_SHARED},
		"slave":         {false, syscall.MS_SLAVE},
		"strictatime":   {false, syscall.MS_STRICTATIME},
		"suid":          {true, syscall.MS_NOSUID},
		"sync":          {false, syscall.MS_SYNCHRONOUS},
		"unbindable":    {false, syscall.MS_UNBINDABLE},
	}
	for _, o := range strings.Split(options, ",") {
		// If the option does not exist in the flags table or the flag
		// is not supported on the platform,
		// then it is a data value for a specific fs type
		if f, exists := flags[o]; exists && f.flag != 0 {
			if f.clear {
				flag &= ^f.flag
			} else {
				flag |= f.flag
			}
		} else {
			data = append(data, o)
		}
	}
	return flag, strings.Join(data, ",")
}

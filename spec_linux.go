// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
)

type UserMapping struct {
	From  int `json:"from"`
	To    int `json:"to"`
	Count int `json:"count"`
}

type Rlimit struct {
	Type int    `json:"type"`
	Hard uint64 `json:"hard"`
	Soft uint64 `json:"soft"`
}

type HugepageLimit struct {
	Pagesize string `json:"pageSize"`
	Limit    int    `json:"limit"`
}

type IfPrioMap struct {
	Interface string `json:"interface"`
	Priority  int64  `json:"priority"`
}

type Resources struct {
	// Memory reservation or soft_limit (in bytes)
	MemoryReservation int64 `json:"memoryReservation"`
	// Total memory usage (memory + swap); set `-1' to disable swap
	MemorySwap int64 `json:"memorySwap"`
	// Kernel memory limit (in bytes)
	KernelMemory int64 `json:"kernelMemory"`
	// CPU shares (relative weight vs. other containers)
	CpuShares int64 `json:"cpuShares"`
	// CPU hardcap limit (in usecs). Allowed cpu time in a given period.
	CpuQuota int64 `json:"cpuQuota"`
	// CPU period to be used for hardcapping (in usecs). 0 to use system default.
	CpuPeriod int64 `json:"cpuPeriod"`
	// How many time CPU will use in realtime scheduling (in usecs).
	CpuRtRuntime int64 `json:"cpuQuota"`
	// CPU period to be used for realtime scheduling (in usecs).
	CpuRtPeriod int64 `json:"cpuPeriod"`
	// CPU to use
	CpusetCpus string `json:"cpusetCpus"`
	// MEM to use
	CpusetMems string `json:"cpusetMems"`
	// IO read rate limit per cgroup per device, bytes per second.
	BlkioThrottleReadBpsDevice string `json:"blkioThrottleReadBpsDevice"`
	// IO write rate limit per cgroup per divice, bytes per second.
	BlkioThrottleWriteBpsDevice string `json:"blkioThrottleWriteBpsDevice"`
	// IO read rate limit per cgroup per device, IO per second.
	BlkioThrottleReadIOpsDevice string `json:"blkioThrottleReadIopsDevice"`
	// IO write rate limit per cgroup per device, IO per second.
	BlkioThrottleWriteIOpsDevice string `json:"blkioThrottleWriteIopsDevice"`
	// Specifies per cgroup weight, range is from 10 to 1000.
	BlkioWeight int64 `json:"blkioWeight"`
	// Weight per cgroup per device, can override BlkioWeight.
	BlkioWeightDevice string `json:"blkioWeightDevice"`
	// Hugetlb limit (in bytes)
	HugetlbLimit []*HugepageLimit `json:"hugetlbLimit"`
	// Whether to disable OOM Killer
	DisableOOMKiller bool `json:"disableOOMKiller"`
	// Set priority of network traffic for container
	NetPrioIfpriomap []*IfPrioMap `json:"netPrioIfpriomap"`
	// Set class identifier for container's network packets
	NetClsClassid string `json:"netClsClassid"`
}

type LinuxSpec struct {
	PortableSpec
	UserMapping      map[string]UserMapping `json:"userMapping"`
	Rlimits          []Rlimit               `json:"rlimits"`
	SystemProperties map[string]string      `json:"systemProperties"`
	Resources        *Resources             `json:"resources"`
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
// If the path is empty then the default path will be "container.json"
func loadSpec(path string) (*LinuxSpec, error) {
	if path == "" {
		path = "container.json"
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file for %s not found", path)
		}
		return nil, err
	}
	defer f.Close()
	var s *LinuxSpec
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return s, nil
}

func createLibcontainerConfig(spec *LinuxSpec) (*configs.Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	config := &configs.Config{
		Capabilities: spec.Capabilities,
		Rootfs:       filepath.Join(cwd, spec.Root.Path),
		Readonlyfs:   spec.Root.Readonly,
		Hostname:     spec.Hostname,
		Privatefs:    true,
	}
	for _, ns := range spec.Namespaces {
		t, exists := namespaceMapping[ns.Type]
		if !exists {
			return nil, fmt.Errorf("namespace %q does not exist", ns)
		}
		config.Namespaces.Add(t, ns.Path)
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
	return config, nil
}

func createLibcontainerMount(cwd string, m Mount) *configs.Mount {
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

func createCgroupConfig(spec *LinuxSpec, devices []*configs.Device) (*configs.Cgroup, error) {
	myCgroupPath, err := cgroups.GetThisCgroupDir("devices")
	if err != nil {
		return nil, err
	}
	c := &configs.Cgroup{
		Name:             getDefaultID(),
		Parent:           myCgroupPath,
		AllowedDevices:   append(devices, allowedDevices...),
		CpuQuota:         getCPUQuota(spec.Cpus),
		Memory:           spec.Memory * 1024 * 1024,
		MemorySwap:       -1,
		MemorySwappiness: -1,
	}
	if r := spec.Resources; r != nil {
		c.MemoryReservation = r.MemoryReservation
		c.MemorySwap = r.MemorySwap
		c.KernelMemory = r.KernelMemory
		c.CpuShares = r.CpuShares
		c.CpuQuota = r.CpuQuota
		c.CpuPeriod = r.CpuPeriod
		c.CpuRtRuntime = r.CpuRtRuntime
		c.CpuRtPeriod = r.CpuRtPeriod
		c.CpusetCpus = r.CpusetCpus
		c.CpusetMems = r.CpusetMems
		c.BlkioThrottleReadBpsDevice = r.BlkioThrottleReadBpsDevice
		c.BlkioThrottleWriteBpsDevice = r.BlkioThrottleWriteBpsDevice
		c.BlkioThrottleReadIOpsDevice = r.BlkioThrottleReadIOpsDevice
		c.BlkioThrottleWriteIOpsDevice = r.BlkioThrottleWriteIOpsDevice
		c.BlkioWeight = r.BlkioWeight
		c.BlkioWeightDevice = r.BlkioWeightDevice
		for _, l := range r.HugetlbLimit {
			c.HugetlbLimit = append(c.HugetlbLimit, &configs.HugepageLimit{
				Pagesize: l.Pagesize,
				Limit:    l.Limit,
			})
		}
		c.OomKillDisable = r.DisableOOMKiller
		for _, m := range r.NetPrioIfpriomap {
			c.NetPrioIfpriomap = append(c.NetPrioIfpriomap, &configs.IfPrioMap{
				Interface: m.Interface,
				Priority:  m.Priority,
			})
		}
		c.NetClsClassid = r.NetClsClassid
	}
	return c, nil
}

func createDevices(spec *LinuxSpec, config *configs.Config) error {
	for _, name := range spec.Devices {
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

func getCPUQuota(cpus float64) int64 {
	return int64(cpus * cpuQuotaMultiplyer)
}

func setupUserNamespace(spec *LinuxSpec, config *configs.Config) error {
	if len(spec.UserMapping) == 0 {
		return nil
	}
	config.Namespaces.Add(configs.NEWUSER, "")
	mappings := make(map[string][]configs.IDMap)
	for k, v := range spec.UserMapping {
		mappings[k] = append(mappings[k], configs.IDMap{
			ContainerID: v.From,
			HostID:      v.To,
			Size:        v.Count,
		})
	}
	config.UidMappings = mappings["uid"]
	config.GidMappings = mappings["gid"]
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
		"defaults":      {false, 0},
		"ro":            {false, syscall.MS_RDONLY},
		"rw":            {true, syscall.MS_RDONLY},
		"suid":          {true, syscall.MS_NOSUID},
		"nosuid":        {false, syscall.MS_NOSUID},
		"dev":           {true, syscall.MS_NODEV},
		"nodev":         {false, syscall.MS_NODEV},
		"exec":          {true, syscall.MS_NOEXEC},
		"noexec":        {false, syscall.MS_NOEXEC},
		"sync":          {false, syscall.MS_SYNCHRONOUS},
		"async":         {true, syscall.MS_SYNCHRONOUS},
		"dirsync":       {false, syscall.MS_DIRSYNC},
		"remount":       {false, syscall.MS_REMOUNT},
		"mand":          {false, syscall.MS_MANDLOCK},
		"nomand":        {true, syscall.MS_MANDLOCK},
		"atime":         {true, syscall.MS_NOATIME},
		"noatime":       {false, syscall.MS_NOATIME},
		"diratime":      {true, syscall.MS_NODIRATIME},
		"nodiratime":    {false, syscall.MS_NODIRATIME},
		"bind":          {false, syscall.MS_BIND},
		"rbind":         {false, syscall.MS_BIND | syscall.MS_REC},
		"unbindable":    {false, syscall.MS_UNBINDABLE},
		"runbindable":   {false, syscall.MS_UNBINDABLE | syscall.MS_REC},
		"private":       {false, syscall.MS_PRIVATE},
		"rprivate":      {false, syscall.MS_PRIVATE | syscall.MS_REC},
		"shared":        {false, syscall.MS_SHARED},
		"rshared":       {false, syscall.MS_SHARED | syscall.MS_REC},
		"slave":         {false, syscall.MS_SLAVE},
		"rslave":        {false, syscall.MS_SLAVE | syscall.MS_REC},
		"relatime":      {false, syscall.MS_RELATIME},
		"norelatime":    {true, syscall.MS_RELATIME},
		"strictatime":   {false, syscall.MS_STRICTATIME},
		"nostrictatime": {true, syscall.MS_STRICTATIME},
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

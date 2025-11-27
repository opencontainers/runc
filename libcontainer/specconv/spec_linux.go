// Package specconv implements conversion of specifications to libcontainer
// configurations
package specconv

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	dbus "github.com/godbus/dbus/v5"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/cgroups"
	devices "github.com/opencontainers/cgroups/devices/config"
	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/internal/pathrs"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/internal/userns"
	"github.com/opencontainers/runc/libcontainer/seccomp"
)

var (
	initMapsOnce            sync.Once
	namespaceMapping        map[specs.LinuxNamespaceType]configs.NamespaceType
	mountPropagationMapping map[string]int
	recAttrFlags            map[string]struct {
		clear bool
		flag  uint64
	}
	mountFlags, extensionFlags map[string]struct {
		clear bool
		flag  int
	}
	complexFlags map[string]func(*configs.Mount)
	mpolModeMap  map[string]uint
	mpolModeFMap map[string]uint
)

func initMaps() {
	initMapsOnce.Do(func() {
		namespaceMapping = map[specs.LinuxNamespaceType]configs.NamespaceType{
			specs.PIDNamespace:     configs.NEWPID,
			specs.NetworkNamespace: configs.NEWNET,
			specs.MountNamespace:   configs.NEWNS,
			specs.UserNamespace:    configs.NEWUSER,
			specs.IPCNamespace:     configs.NEWIPC,
			specs.UTSNamespace:     configs.NEWUTS,
			specs.CgroupNamespace:  configs.NEWCGROUP,
			specs.TimeNamespace:    configs.NEWTIME,
		}

		mountPropagationMapping = map[string]int{
			"rprivate":    unix.MS_PRIVATE | unix.MS_REC,
			"private":     unix.MS_PRIVATE,
			"rslave":      unix.MS_SLAVE | unix.MS_REC,
			"slave":       unix.MS_SLAVE,
			"rshared":     unix.MS_SHARED | unix.MS_REC,
			"shared":      unix.MS_SHARED,
			"runbindable": unix.MS_UNBINDABLE | unix.MS_REC,
			"unbindable":  unix.MS_UNBINDABLE,
		}

		mountFlags = map[string]struct {
			clear bool
			flag  int
		}{
			// "acl" cannot be mapped to MS_POSIXACL: https://github.com/opencontainers/runc/issues/3738
			"async":         {true, unix.MS_SYNCHRONOUS},
			"atime":         {true, unix.MS_NOATIME},
			"bind":          {false, unix.MS_BIND},
			"defaults":      {false, 0},
			"dev":           {true, unix.MS_NODEV},
			"diratime":      {true, unix.MS_NODIRATIME},
			"dirsync":       {false, unix.MS_DIRSYNC},
			"exec":          {true, unix.MS_NOEXEC},
			"iversion":      {false, unix.MS_I_VERSION},
			"lazytime":      {false, unix.MS_LAZYTIME},
			"loud":          {true, unix.MS_SILENT},
			"mand":          {false, unix.MS_MANDLOCK},
			"noatime":       {false, unix.MS_NOATIME},
			"nodev":         {false, unix.MS_NODEV},
			"nodiratime":    {false, unix.MS_NODIRATIME},
			"noexec":        {false, unix.MS_NOEXEC},
			"noiversion":    {true, unix.MS_I_VERSION},
			"nolazytime":    {true, unix.MS_LAZYTIME},
			"nomand":        {true, unix.MS_MANDLOCK},
			"norelatime":    {true, unix.MS_RELATIME},
			"nostrictatime": {true, unix.MS_STRICTATIME},
			"nosuid":        {false, unix.MS_NOSUID},
			"nosymfollow":   {false, unix.MS_NOSYMFOLLOW}, // since kernel 5.10
			"rbind":         {false, unix.MS_BIND | unix.MS_REC},
			"relatime":      {false, unix.MS_RELATIME},
			"remount":       {false, unix.MS_REMOUNT},
			"ro":            {false, unix.MS_RDONLY},
			"rw":            {true, unix.MS_RDONLY},
			"silent":        {false, unix.MS_SILENT},
			"strictatime":   {false, unix.MS_STRICTATIME},
			"suid":          {true, unix.MS_NOSUID},
			"sync":          {false, unix.MS_SYNCHRONOUS},
			"symfollow":     {true, unix.MS_NOSYMFOLLOW}, // since kernel 5.10
		}

		recAttrFlags = map[string]struct {
			clear bool
			flag  uint64
		}{
			"rro":            {false, unix.MOUNT_ATTR_RDONLY},
			"rrw":            {true, unix.MOUNT_ATTR_RDONLY},
			"rnosuid":        {false, unix.MOUNT_ATTR_NOSUID},
			"rsuid":          {true, unix.MOUNT_ATTR_NOSUID},
			"rnodev":         {false, unix.MOUNT_ATTR_NODEV},
			"rdev":           {true, unix.MOUNT_ATTR_NODEV},
			"rnoexec":        {false, unix.MOUNT_ATTR_NOEXEC},
			"rexec":          {true, unix.MOUNT_ATTR_NOEXEC},
			"rnodiratime":    {false, unix.MOUNT_ATTR_NODIRATIME},
			"rdiratime":      {true, unix.MOUNT_ATTR_NODIRATIME},
			"rrelatime":      {false, unix.MOUNT_ATTR_RELATIME},
			"rnorelatime":    {true, unix.MOUNT_ATTR_RELATIME},
			"rnoatime":       {false, unix.MOUNT_ATTR_NOATIME},
			"ratime":         {true, unix.MOUNT_ATTR_NOATIME},
			"rstrictatime":   {false, unix.MOUNT_ATTR_STRICTATIME},
			"rnostrictatime": {true, unix.MOUNT_ATTR_STRICTATIME},
			"rnosymfollow":   {false, unix.MOUNT_ATTR_NOSYMFOLLOW}, // since kernel 5.14
			"rsymfollow":     {true, unix.MOUNT_ATTR_NOSYMFOLLOW},  // since kernel 5.14
		}

		extensionFlags = map[string]struct {
			clear bool
			flag  int
		}{
			"tmpcopyup": {false, configs.EXT_COPYUP},
		}

		complexFlags = map[string]func(*configs.Mount){
			"idmap": func(m *configs.Mount) {
				m.IDMapping = new(configs.MountIDMapping)
				m.IDMapping.Recursive = false // noop
			},
			"ridmap": func(m *configs.Mount) {
				m.IDMapping = new(configs.MountIDMapping)
				m.IDMapping.Recursive = true
			},
		}

		mpolModeMap = map[string]uint{
			string(specs.MpolDefault):            configs.MPOL_DEFAULT,
			string(specs.MpolPreferred):          configs.MPOL_PREFERRED,
			string(specs.MpolBind):               configs.MPOL_BIND,
			string(specs.MpolInterleave):         configs.MPOL_INTERLEAVE,
			string(specs.MpolLocal):              configs.MPOL_LOCAL,
			string(specs.MpolPreferredMany):      configs.MPOL_PREFERRED_MANY,
			string(specs.MpolWeightedInterleave): configs.MPOL_WEIGHTED_INTERLEAVE,
		}

		mpolModeFMap = map[string]uint{
			string(specs.MpolFStaticNodes):   configs.MPOL_F_STATIC_NODES,
			string(specs.MpolFRelativeNodes): configs.MPOL_F_RELATIVE_NODES,
			string(specs.MpolFNumaBalancing): configs.MPOL_F_NUMA_BALANCING,
		}
	})
}

// KnownNamespaces returns the list of the known namespaces.
// Used by `runc features`.
func KnownNamespaces() []string {
	initMaps()
	var res []string
	for k := range namespaceMapping {
		res = append(res, string(k))
	}
	sort.Strings(res)
	return res
}

// KnownMountOptions returns the list of the known mount options.
// Used by `runc features`.
func KnownMountOptions() []string {
	initMaps()
	var res []string
	for k := range mountFlags {
		res = append(res, k)
	}
	for k := range mountPropagationMapping {
		res = append(res, k)
	}
	for k := range recAttrFlags {
		res = append(res, k)
	}
	for k := range extensionFlags {
		res = append(res, k)
	}
	sort.Strings(res)
	return res
}

// KnownMemoryPolicyModes returns the list of the known memory policy modes.
// Used by `runc features`.
func KnownMemoryPolicyModes() []string {
	initMaps()
	return slices.Sorted(maps.Keys(mpolModeMap))
}

// KnownMemoryPolicyFlags returns the list of the known memory policy mode flags.
// Used by `runc features`.
func KnownMemoryPolicyFlags() []string {
	initMaps()
	return slices.Sorted(maps.Keys(mpolModeFMap))
}

// AllowedDevices is the set of devices which are automatically included for
// all containers.
//
// # XXX (cyphar)
//
// This behaviour is at the very least "questionable" (if not outright
// wrong) according to the runtime-spec.
//
// Yes, we have to include certain devices other than the ones the user
// specifies, but several devices listed here are not part of the spec
// (including "mknod for any device"?!). In addition, these rules are
// appended to the user-provided set which means that users *cannot disable
// this behaviour*.
//
// ... unfortunately I'm too scared to change this now because who knows how
// many people depend on this (incorrect and arguably insecure) behaviour.
var AllowedDevices = []*devices.Device{
	// allow mknod for any device
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
	},
	{
		Rule: devices.Rule{
			Type:        devices.BlockDevice,
			Major:       devices.Wildcard,
			Minor:       devices.Wildcard,
			Permissions: "m",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/null",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       3,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/random",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       8,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/full",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       7,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/tty",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       0,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/zero",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       5,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Path:     "/dev/urandom",
		FileMode: 0o666,
		Uid:      0,
		Gid:      0,
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       1,
			Minor:       9,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	// /dev/pts/ - pts namespaces are "coming soon"
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       136,
			Minor:       devices.Wildcard,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       5,
			Minor:       2,
			Permissions: "rwm",
			Allow:       true,
		},
	},
	// The following entry for /dev/net/tun device was there from the
	// very early days of Docker, but got removed in runc 1.2.0-rc1,
	// causing a number of regressions for users (see
	// https://github.com/opencontainers/runc/pull/3468).
	//
	// Some upper-level orcherstration tools makes it either impossible
	// or cumbersome to supply additional device rules, so we have to
	// keep this for the sake of backward compatibility.
	{
		Rule: devices.Rule{
			Type:        devices.CharDevice,
			Major:       10,
			Minor:       200,
			Permissions: "rwm",
			Allow:       true,
		},
	},
}

type CreateOpts struct {
	CgroupName       string
	UseSystemdCgroup bool
	NoPivotRoot      bool
	NoNewKeyring     bool
	Spec             *specs.Spec
	RootlessEUID     bool
	RootlessCgroups  bool
}

// CreateLibcontainerConfig creates a new libcontainer configuration from a
// given specification and a cgroup name
func CreateLibcontainerConfig(opts *CreateOpts) (*configs.Config, error) {
	// Runc's cwd will always be the bundle path.
	// Use the value from the kernel, which guarantees the returned value
	// to be absolute and clean.
	cwd, err := linux.Getwd()
	if err != nil {
		return nil, err
	}
	spec := opts.Spec
	if spec.Root == nil {
		return nil, errors.New("root must be specified")
	}
	rootfsPath := spec.Root.Path
	if !filepath.IsAbs(rootfsPath) {
		rootfsPath = filepath.Join(cwd, rootfsPath)
	}
	labels := []string{}
	for k, v := range spec.Annotations {
		labels = append(labels, k+"="+v)
	}
	config := &configs.Config{
		Rootfs:          rootfsPath,
		NoPivotRoot:     opts.NoPivotRoot,
		Readonlyfs:      spec.Root.Readonly,
		Hostname:        spec.Hostname,
		Domainname:      spec.Domainname,
		Labels:          append(labels, "bundle="+cwd),
		NoNewKeyring:    opts.NoNewKeyring,
		RootlessEUID:    opts.RootlessEUID,
		RootlessCgroups: opts.RootlessCgroups,
	}

	for _, m := range spec.Mounts {
		cm, err := createLibcontainerMount(cwd, m)
		if err != nil {
			return nil, fmt.Errorf("invalid mount %+v: %w", m, err)
		}
		config.Mounts = append(config.Mounts, cm)
	}

	defaultDevs, err := createDevices(spec, config)
	if err != nil {
		return nil, err
	}

	c, err := CreateCgroupConfig(opts, defaultDevs)
	if err != nil {
		return nil, err
	}

	config.Cgroups = c
	// set linux-specific config
	if spec.Linux != nil {
		initMaps()

		if spec.Linux.RootfsPropagation != "" {
			var exists bool
			if config.RootPropagation, exists = mountPropagationMapping[spec.Linux.RootfsPropagation]; !exists {
				return nil, fmt.Errorf("rootfsPropagation=%v is not supported", spec.Linux.RootfsPropagation)
			}
			if config.NoPivotRoot && (config.RootPropagation&unix.MS_PRIVATE != 0) {
				return nil, errors.New("rootfsPropagation of [r]private is not safe without pivot_root")
			}
		}

		for _, ns := range spec.Linux.Namespaces {
			t, exists := namespaceMapping[ns.Type]
			if !exists {
				return nil, fmt.Errorf("namespace %q does not exist", ns)
			}
			if config.Namespaces.Contains(t) {
				return nil, fmt.Errorf("malformed spec file: duplicated ns %q", ns)
			}
			config.Namespaces.Add(t, ns.Path)
		}
		if config.Namespaces.IsPrivate(configs.NEWNET) {
			config.Networks = []*configs.Network{
				{
					Type: "loopback",
				},
			}
		}
		if config.Namespaces.Contains(configs.NEWUSER) {
			if err := setupUserNamespace(spec, config); err != nil {
				return nil, err
			}
			// For idmap and ridmap mounts without explicit mappings, use the
			// ones from the container's userns. If we are joining another
			// userns, stash the path.
			for _, m := range config.Mounts {
				if m.IDMapping != nil && m.IDMapping.UIDMappings == nil && m.IDMapping.GIDMappings == nil {
					if path := config.Namespaces.PathOf(configs.NEWUSER); path != "" {
						m.IDMapping.UserNSPath = path
					} else {
						m.IDMapping.UIDMappings = config.UIDMappings
						m.IDMapping.GIDMappings = config.GIDMappings
					}
				}
			}
		}
		config.MaskPaths = spec.Linux.MaskedPaths
		config.ReadonlyPaths = spec.Linux.ReadonlyPaths
		config.MountLabel = spec.Linux.MountLabel
		config.Sysctl = spec.Linux.Sysctl
		config.TimeOffsets = spec.Linux.TimeOffsets
		if spec.Linux.Seccomp != nil {
			seccomp, err := SetupSeccomp(spec.Linux.Seccomp)
			if err != nil {
				return nil, err
			}
			config.Seccomp = seccomp
		}
		if spec.Linux.IntelRdt != nil {
			config.IntelRdt = &configs.IntelRdt{
				ClosID:           spec.Linux.IntelRdt.ClosID,
				Schemata:         spec.Linux.IntelRdt.Schemata,
				L3CacheSchema:    spec.Linux.IntelRdt.L3CacheSchema,
				MemBwSchema:      spec.Linux.IntelRdt.MemBwSchema,
				EnableMonitoring: spec.Linux.IntelRdt.EnableMonitoring,
			}
		}
		if spec.Linux.MemoryPolicy != nil {
			var ok bool
			var err error
			specMp := spec.Linux.MemoryPolicy
			confMp := &configs.LinuxMemoryPolicy{}
			confMp.Mode, ok = mpolModeMap[string(specMp.Mode)]
			if !ok {
				return nil, fmt.Errorf("invalid memory policy mode %q", specMp.Mode)
			}
			confMp.Nodes, err = configs.ToCPUSet(specMp.Nodes)
			if err != nil {
				return nil, fmt.Errorf("invalid memory policy nodes %q: %w", specMp.Nodes, err)
			}
			for _, specFlag := range specMp.Flags {
				confFlag, ok := mpolModeFMap[string(specFlag)]
				if !ok {
					return nil, fmt.Errorf("invalid memory policy flag %q", specFlag)
				}
				confMp.Flags |= confFlag
			}
			config.MemoryPolicy = confMp
		}
		if spec.Linux.Personality != nil {
			if len(spec.Linux.Personality.Flags) > 0 {
				logrus.Warnf("ignoring unsupported personality flags: %+v because personality flag has not supported at this time", spec.Linux.Personality.Flags)
			}
			domain, err := getLinuxPersonalityFromStr(string(spec.Linux.Personality.Domain))
			if err != nil {
				return nil, err
			}
			config.Personality = &configs.LinuxPersonality{
				Domain: domain,
			}
		}

		for name, netdev := range spec.Linux.NetDevices {
			if config.NetDevices == nil {
				config.NetDevices = make(map[string]*configs.LinuxNetDevice)
			}
			config.NetDevices[name] = &configs.LinuxNetDevice{
				Name: netdev.Name,
			}
		}
	}

	// Set the host UID that should own the container's cgroup.
	// This must be performed after setupUserNamespace, so that
	// config.HostRootUID() returns the correct result.
	//
	// Only set it if the container will have its own cgroup
	// namespace and the cgroupfs will be mounted read/write.
	//
	hasCgroupNS := config.Namespaces.IsPrivate(configs.NEWCGROUP)
	hasRwCgroupfs := false
	if hasCgroupNS {
		for _, m := range config.Mounts {
			if m.Source == "cgroup" && filepath.Clean(m.Destination) == "/sys/fs/cgroup" && (m.Flags&unix.MS_RDONLY) == 0 {
				hasRwCgroupfs = true
				break
			}
		}
	}
	processUid := 0
	if spec.Process != nil {
		// Chown the cgroup to the UID running the process,
		// which is not necessarily UID 0 in the container
		// namespace (e.g., an unprivileged UID in the host
		// user namespace).
		processUid = int(spec.Process.User.UID)
	}
	if hasCgroupNS && hasRwCgroupfs {
		ownerUid, err := config.HostUID(processUid)
		// There are two error cases; we can ignore both.
		//
		// 1. uidMappings is unset.  Either there is no user
		//    namespace (fine), or it is an error (which is
		//    checked elsewhere).
		//
		// 2. The user is unmapped in the user namespace.  This is an
		//    unusual configuration and might be an error.  But it too
		//    will be checked elsewhere, so we can ignore it here.
		//
		if err == nil {
			config.Cgroups.OwnerUID = &ownerUid
		}
	}

	if spec.Process != nil {
		config.OomScoreAdj = spec.Process.OOMScoreAdj
		config.NoNewPrivileges = spec.Process.NoNewPrivileges
		config.Umask = spec.Process.User.Umask
		config.ProcessLabel = spec.Process.SelinuxLabel
		if spec.Process.Capabilities != nil {
			config.Capabilities = &configs.Capabilities{
				Bounding:    spec.Process.Capabilities.Bounding,
				Effective:   spec.Process.Capabilities.Effective,
				Permitted:   spec.Process.Capabilities.Permitted,
				Inheritable: spec.Process.Capabilities.Inheritable,
				Ambient:     spec.Process.Capabilities.Ambient,
			}
		}
		if spec.Process.Scheduler != nil {
			s := *spec.Process.Scheduler
			config.Scheduler = &s
		}

		if spec.Process.IOPriority != nil {
			ioPriority := *spec.Process.IOPriority
			config.IOPriority = &ioPriority
		}
		config.ExecCPUAffinity, err = configs.ConvertCPUAffinity(spec.Process.ExecCPUAffinity)
		if err != nil {
			return nil, err
		}

	}
	createHooks(spec, config)
	config.Version = specs.Version
	return config, nil
}

func toConfigIDMap(specMaps []specs.LinuxIDMapping) []configs.IDMap {
	if specMaps == nil {
		return nil
	}
	idmaps := make([]configs.IDMap, len(specMaps))
	for i, id := range specMaps {
		idmaps[i] = configs.IDMap{
			ContainerID: int64(id.ContainerID),
			HostID:      int64(id.HostID),
			Size:        int64(id.Size),
		}
	}
	return idmaps
}

func createLibcontainerMount(cwd string, m specs.Mount) (*configs.Mount, error) {
	if !filepath.IsAbs(m.Destination) {
		// Relax validation for backward compatibility
		// TODO (runc v1.x.x): change warning to an error
		// return nil, fmt.Errorf("mount destination %s is not absolute", m.Destination)
		logrus.Warnf("mount destination %s is not absolute. Support for non-absolute mount destinations will be removed in a future release.", m.Destination)
	}
	mnt := parseMountOptions(m.Options)

	mnt.Destination = m.Destination
	mnt.Source = m.Source
	mnt.Device = m.Type
	if mnt.Flags&unix.MS_BIND != 0 {
		// Any "type" the user specified is meaningless (and ignored) for
		// bind-mounts -- so we set it to "bind" because rootfs_linux.go
		// (incorrectly) relies on this for some checks.
		mnt.Device = "bind"
		if !filepath.IsAbs(mnt.Source) {
			mnt.Source = filepath.Join(cwd, m.Source)
		}
	}

	if m.UIDMappings != nil || m.GIDMappings != nil {
		if mnt.IDMapping == nil {
			// Neither "idmap" nor "ridmap" were specified.
			mnt.IDMapping = new(configs.MountIDMapping)
		}
		mnt.IDMapping.UIDMappings = toConfigIDMap(m.UIDMappings)
		mnt.IDMapping.GIDMappings = toConfigIDMap(m.GIDMappings)
	}

	// None of the mount arguments can contain a null byte. Normally such
	// strings would either cause some other failure or would just be truncated
	// when we hit the null byte, but because we serialise these strings as
	// netlink messages (which don't have special null-byte handling) we need
	// to block this as early as possible.
	if strings.IndexByte(mnt.Source, 0) >= 0 ||
		strings.IndexByte(mnt.Destination, 0) >= 0 ||
		strings.IndexByte(mnt.Device, 0) >= 0 {
		return nil, errors.New("mount field contains null byte")
	}

	return mnt, nil
}

// checkPropertyName checks if systemd property name is valid. A valid name
// should consist of latin letters only, and have least 3 of them.
func checkPropertyName(s string) error {
	if len(s) < 3 {
		return errors.New("too short")
	}
	// Check ASCII characters rather than Unicode runes,
	// so we have to use indexes rather than range.
	for i := range len(s) {
		ch := s[i]
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			continue
		}
		return errors.New("contains non-alphabetic character")
	}
	return nil
}

// getLinuxPersonalityFromStr converts the string domain received from spec to equivalent integer.
func getLinuxPersonalityFromStr(domain string) (int, error) {
	if domain == string(specs.PerLinux32) {
		return configs.PerLinux32, nil
	} else if domain == string(specs.PerLinux) {
		return configs.PerLinux, nil
	}
	return -1, fmt.Errorf("invalid personality domain %s", domain)
}

// Some systemd properties are documented as having "Sec" suffix
// (e.g. TimeoutStopSec) but are expected to have "USec" suffix
// here, so let's provide conversion to improve compatibility.
func convertSecToUSec(value dbus.Variant) (dbus.Variant, error) {
	var sec uint64
	const M = 1000000
	vi := value.Value()
	switch value.Signature().String() {
	case "y":
		sec = uint64(vi.(byte)) * M
	case "n":
		sec = uint64(vi.(int16)) * M
	case "q":
		sec = uint64(vi.(uint16)) * M
	case "i":
		sec = uint64(vi.(int32)) * M
	case "u":
		sec = uint64(vi.(uint32)) * M
	case "x":
		sec = uint64(vi.(int64)) * M
	case "t":
		sec = vi.(uint64) * M
	case "d":
		sec = uint64(vi.(float64) * M)
	default:
		return value, errors.New("not a number")
	}
	return dbus.MakeVariant(sec), nil
}

func initSystemdProps(spec *specs.Spec) ([]systemdDbus.Property, error) {
	const keyPrefix = "org.systemd.property."
	var sp []systemdDbus.Property

	for k, v := range spec.Annotations {
		name, ok := strings.CutPrefix(k, keyPrefix)
		if !ok { // prefix not there
			continue
		}
		if err := checkPropertyName(name); err != nil {
			return nil, fmt.Errorf("annotation %s name incorrect: %w", k, err)
		}
		value, err := dbus.ParseVariant(v, dbus.Signature{})
		if err != nil {
			return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
		}
		// Check for Sec suffix.
		if trimName := strings.TrimSuffix(name, "Sec"); len(trimName) < len(name) {
			// Check for a lowercase ascii a-z just before Sec.
			if ch := trimName[len(trimName)-1]; ch >= 'a' && ch <= 'z' {
				// Convert from Sec to USec.
				name = trimName + "USec"
				value, err = convertSecToUSec(value)
				if err != nil {
					return nil, fmt.Errorf("annotation %s=%s value parse error: %w", k, v, err)
				}
			}
		}
		sp = append(sp, systemdDbus.Property{Name: name, Value: value})
	}

	return sp, nil
}

func CreateCgroupConfig(opts *CreateOpts, defaultDevs []*devices.Device) (*cgroups.Cgroup, error) {
	var (
		myCgroupPath string

		spec             = opts.Spec
		useSystemdCgroup = opts.UseSystemdCgroup
		name             = opts.CgroupName
	)

	c := &cgroups.Cgroup{
		Systemd:   useSystemdCgroup,
		Rootless:  opts.RootlessCgroups,
		Resources: &cgroups.Resources{},
	}

	if useSystemdCgroup {
		sp, err := initSystemdProps(spec)
		if err != nil {
			return nil, err
		}
		c.SystemdProps = sp
	}

	if spec.Linux != nil && spec.Linux.CgroupsPath != "" {
		if useSystemdCgroup {
			myCgroupPath = spec.Linux.CgroupsPath
		} else {
			myCgroupPath = pathrs.LexicallyCleanPath(spec.Linux.CgroupsPath)
		}
	}

	if useSystemdCgroup {
		if myCgroupPath == "" {
			// Default for c.Parent is set by systemd cgroup drivers.
			c.ScopePrefix = "runc"
			c.Name = name
		} else {
			// Parse the path from expected "slice:prefix:name"
			// for e.g. "system.slice:docker:1234"
			parts := strings.Split(myCgroupPath, ":")
			if len(parts) != 3 {
				return nil, fmt.Errorf("expected cgroupsPath to be of format \"slice:prefix:name\" for systemd cgroups, got %q instead", myCgroupPath)
			}
			c.Parent = parts[0]
			c.ScopePrefix = parts[1]
			c.Name = parts[2]
		}
	} else {
		if myCgroupPath == "" {
			c.Name = name
		}
		c.Path = myCgroupPath
	}

	// In rootless containers, any attempt to make cgroup changes is likely to fail.
	// libcontainer will validate this but ignores the error.
	if spec.Linux != nil {
		r := spec.Linux.Resources
		if r != nil {
			for i, d := range r.Devices {
				var (
					t     = "a"
					major = int64(-1)
					minor = int64(-1)
				)
				if d.Type != "" {
					t = d.Type
				}
				if d.Major != nil {
					major = *d.Major
				}
				if d.Minor != nil {
					minor = *d.Minor
				}
				if d.Access == "" {
					return nil, fmt.Errorf("device access at %d field cannot be empty", i)
				}
				dt, err := stringToCgroupDeviceRune(t)
				if err != nil {
					return nil, err
				}
				c.Resources.Devices = append(c.Resources.Devices, &devices.Rule{
					Type:        dt,
					Major:       major,
					Minor:       minor,
					Permissions: devices.Permissions(d.Access),
					Allow:       d.Allow,
				})
			}
			if r.Memory != nil {
				if r.Memory.Limit != nil {
					c.Resources.Memory = *r.Memory.Limit
				}
				if r.Memory.Reservation != nil {
					c.Resources.MemoryReservation = *r.Memory.Reservation
				}
				if r.Memory.Swap != nil {
					c.Resources.MemorySwap = *r.Memory.Swap
				}
				if r.Memory.Kernel != nil || r.Memory.KernelTCP != nil { //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
					logrus.Warn("Kernel memory settings are ignored and will be removed")
				}
				if r.Memory.Swappiness != nil {
					c.Resources.MemorySwappiness = r.Memory.Swappiness
				}
				if r.Memory.DisableOOMKiller != nil {
					c.Resources.OomKillDisable = *r.Memory.DisableOOMKiller
				}
				if r.Memory.CheckBeforeUpdate != nil {
					c.Resources.MemoryCheckBeforeUpdate = *r.Memory.CheckBeforeUpdate
				}
			}
			if r.CPU != nil {
				if r.CPU.Shares != nil {
					c.Resources.CpuShares = *r.CPU.Shares

					// CpuWeight is used for cgroupv2 and should be converted
					c.Resources.CpuWeight = cgroups.ConvertCPUSharesToCgroupV2Value(c.Resources.CpuShares)
				}
				if r.CPU.Quota != nil {
					c.Resources.CpuQuota = *r.CPU.Quota
				}
				if r.CPU.Burst != nil {
					c.Resources.CpuBurst = r.CPU.Burst
				}
				if r.CPU.Period != nil {
					c.Resources.CpuPeriod = *r.CPU.Period
				}
				if r.CPU.RealtimeRuntime != nil {
					c.Resources.CpuRtRuntime = *r.CPU.RealtimeRuntime
				}
				if r.CPU.RealtimePeriod != nil {
					c.Resources.CpuRtPeriod = *r.CPU.RealtimePeriod
				}
				c.Resources.CpusetCpus = r.CPU.Cpus
				c.Resources.CpusetMems = r.CPU.Mems
				c.Resources.CPUIdle = r.CPU.Idle
			}
			if r.Pids != nil {
				c.Resources.PidsLimit = r.Pids.Limit
			}
			if r.BlockIO != nil {
				if r.BlockIO.Weight != nil {
					c.Resources.BlkioWeight = *r.BlockIO.Weight
				}
				if r.BlockIO.LeafWeight != nil {
					c.Resources.BlkioLeafWeight = *r.BlockIO.LeafWeight
				}
				for _, wd := range r.BlockIO.WeightDevice {
					var weight, leafWeight uint16
					if wd.Weight != nil {
						weight = *wd.Weight
					}
					if wd.LeafWeight != nil {
						leafWeight = *wd.LeafWeight
					}
					weightDevice := cgroups.NewWeightDevice(wd.Major, wd.Minor, weight, leafWeight)
					c.Resources.BlkioWeightDevice = append(c.Resources.BlkioWeightDevice, weightDevice)
				}
				for _, td := range r.BlockIO.ThrottleReadBpsDevice {
					rate := td.Rate
					throttleDevice := cgroups.NewThrottleDevice(td.Major, td.Minor, rate)
					c.Resources.BlkioThrottleReadBpsDevice = append(c.Resources.BlkioThrottleReadBpsDevice, throttleDevice)
				}
				for _, td := range r.BlockIO.ThrottleWriteBpsDevice {
					rate := td.Rate
					throttleDevice := cgroups.NewThrottleDevice(td.Major, td.Minor, rate)
					c.Resources.BlkioThrottleWriteBpsDevice = append(c.Resources.BlkioThrottleWriteBpsDevice, throttleDevice)
				}
				for _, td := range r.BlockIO.ThrottleReadIOPSDevice {
					rate := td.Rate
					throttleDevice := cgroups.NewThrottleDevice(td.Major, td.Minor, rate)
					c.Resources.BlkioThrottleReadIOPSDevice = append(c.Resources.BlkioThrottleReadIOPSDevice, throttleDevice)
				}
				for _, td := range r.BlockIO.ThrottleWriteIOPSDevice {
					rate := td.Rate
					throttleDevice := cgroups.NewThrottleDevice(td.Major, td.Minor, rate)
					c.Resources.BlkioThrottleWriteIOPSDevice = append(c.Resources.BlkioThrottleWriteIOPSDevice, throttleDevice)
				}
			}
			for _, l := range r.HugepageLimits {
				c.Resources.HugetlbLimit = append(c.Resources.HugetlbLimit, &cgroups.HugepageLimit{
					Pagesize: l.Pagesize,
					Limit:    l.Limit,
				})
			}
			if len(r.Rdma) > 0 {
				c.Resources.Rdma = make(map[string]cgroups.LinuxRdma, len(r.Rdma))
				for k, v := range r.Rdma {
					c.Resources.Rdma[k] = cgroups.LinuxRdma{
						HcaHandles: v.HcaHandles,
						HcaObjects: v.HcaObjects,
					}
				}
			}
			if r.Network != nil {
				if r.Network.ClassID != nil {
					c.Resources.NetClsClassid = *r.Network.ClassID
				}
				for _, m := range r.Network.Priorities {
					c.Resources.NetPrioIfpriomap = append(c.Resources.NetPrioIfpriomap, &cgroups.IfPrioMap{
						Interface: m.Name,
						Priority:  int64(m.Priority),
					})
				}
			}
			if len(r.Unified) > 0 {
				c.Resources.Unified = maps.Clone(r.Unified)
			}
		}
	}

	// Append the default allowed devices to the end of the list.
	for _, device := range defaultDevs {
		c.Resources.Devices = append(c.Resources.Devices, &device.Rule)
	}
	return c, nil
}

func stringToCgroupDeviceRune(s string) (devices.Type, error) {
	switch s {
	case "a":
		return devices.WildcardDevice, nil
	case "b":
		return devices.BlockDevice, nil
	case "c":
		return devices.CharDevice, nil
	default:
		return 0, fmt.Errorf("invalid cgroup device type %q", s)
	}
}

func stringToDeviceRune(s string) (devices.Type, error) {
	switch s {
	case "p":
		return devices.FifoDevice, nil
	case "u", "c":
		return devices.CharDevice, nil
	case "b":
		return devices.BlockDevice, nil
	default:
		return 0, fmt.Errorf("invalid device type %q", s)
	}
}

func createDevices(spec *specs.Spec, config *configs.Config) ([]*devices.Device, error) {
	// If a spec device is redundant with a default device, remove that default
	// device (the spec one takes priority).
	dedupedAllowDevs := []*devices.Device{}

next:
	for _, ad := range AllowedDevices {
		if ad.Path != "" && spec.Linux != nil {
			for _, sd := range spec.Linux.Devices {
				if sd.Path == ad.Path {
					continue next
				}
			}
		}
		dedupedAllowDevs = append(dedupedAllowDevs, ad)
		if ad.Path != "" {
			config.Devices = append(config.Devices, ad)
		}
	}

	// Merge in additional devices from the spec.
	if spec.Linux != nil {
		for _, d := range spec.Linux.Devices {
			var uid, gid uint32
			var filemode os.FileMode = 0o666

			if d.UID != nil {
				uid = *d.UID
			}
			if d.GID != nil {
				gid = *d.GID
			}
			dt, err := stringToDeviceRune(d.Type)
			if err != nil {
				return nil, err
			}
			if d.FileMode != nil {
				filemode = *d.FileMode &^ unix.S_IFMT
			}
			device := &devices.Device{
				Rule: devices.Rule{
					Type:  dt,
					Major: d.Major,
					Minor: d.Minor,
				},
				Path:     d.Path,
				FileMode: filemode,
				Uid:      uid,
				Gid:      gid,
			}
			config.Devices = append(config.Devices, device)
		}
	}

	return dedupedAllowDevs, nil
}

func setupUserNamespace(spec *specs.Spec, config *configs.Config) error {
	if spec.Linux != nil {
		config.UIDMappings = toConfigIDMap(spec.Linux.UIDMappings)
		config.GIDMappings = toConfigIDMap(spec.Linux.GIDMappings)
	}
	if path := config.Namespaces.PathOf(configs.NEWUSER); path != "" {
		// Cache the current userns mappings in our configuration, so that we
		// can calculate uid and gid mappings within runc. These mappings are
		// never used for configuring the container if the path is set.
		uidMap, gidMap, err := userns.GetUserNamespaceMappings(path)
		if err != nil {
			return fmt.Errorf("failed to cache mappings for userns: %w", err)
		}
		// We cannot allow uid or gid mappings to be set if we are also asked
		// to join a userns.
		if config.UIDMappings != nil || config.GIDMappings != nil {
			// FIXME: It turns out that containerd and CRIO pass both a userns
			// path and the mappings of the namespace in the same config.json.
			// Such a configuration is technically not valid, but we used to
			// require mappings be specified, and thus users worked around our
			// bug -- so we can't regress it at the moment. But we also don't
			// want to produce broken behaviour if the mapping doesn't match
			// the userns. So (for now) we output a warning if the actual
			// userns mappings match the configuration, otherwise we return an
			// error.
			if !userns.IsSameMapping(uidMap, config.UIDMappings) ||
				!userns.IsSameMapping(gidMap, config.GIDMappings) {
				return errors.New("user namespaces enabled, but both namespace path and non-matching mapping specified -- you may only provide one")
			}
			logrus.Warnf("config.json has both a userns path to join and a matching userns mapping specified -- you may only provide one. Future versions of runc may return an error with this configuration, please report a bug on <https://github.com/opencontainers/runc> if you see this warning and cannot update your configuration.")
		}

		config.UIDMappings = uidMap
		config.GIDMappings = gidMap
		logrus.WithFields(logrus.Fields{
			"uid_map": uidMap,
			"gid_map": gidMap,
		}).Debugf("config uses path-based userns configuration -- current uid and gid mappings cached")
	}
	rootUID, err := config.HostRootUID()
	if err != nil {
		return err
	}
	rootGID, err := config.HostRootGID()
	if err != nil {
		return err
	}
	for _, node := range config.Devices {
		node.Uid = uint32(rootUID)
		node.Gid = uint32(rootGID)
	}
	return nil
}

// parseMountOptions parses options and returns a configs.Mount
// structure with fields that depends on options set accordingly.
func parseMountOptions(options []string) *configs.Mount {
	var (
		data                   []string
		m                      configs.Mount
		recAttrSet, recAttrClr uint64
	)
	initMaps()
	for _, o := range options {
		// If the option does not exist in the mountFlags table,
		// or the flag is not supported on the platform,
		// then it is a data value for a specific fs type.
		if f, exists := mountFlags[o]; exists && f.flag != 0 {
			// FIXME: The *atime flags are special (they are more of an enum
			// with quite hairy semantics) and thus arguably setting some of
			// them should clear unrelated flags.
			if f.clear {
				m.Flags &= ^f.flag
				m.ClearedFlags |= f.flag
			} else {
				m.Flags |= f.flag
				m.ClearedFlags &= ^f.flag
			}
		} else if f, exists := mountPropagationMapping[o]; exists && f != 0 {
			m.PropagationFlags = append(m.PropagationFlags, f)
		} else if f, exists := recAttrFlags[o]; exists {
			if f.clear {
				recAttrClr |= f.flag
				recAttrSet &= ^f.flag
			} else {
				recAttrSet |= f.flag
				recAttrClr &= ^f.flag
				if f.flag&unix.MOUNT_ATTR__ATIME == f.flag {
					// https://man7.org/linux/man-pages/man2/mount_setattr.2.html
					// "cannot simply specify the access-time setting in attr_set, but must also include MOUNT_ATTR__ATIME in the attr_clr field."
					recAttrClr |= unix.MOUNT_ATTR__ATIME
				}
			}
		} else if f, exists := extensionFlags[o]; exists {
			if f.clear {
				m.Extensions &= ^f.flag
			} else {
				m.Extensions |= f.flag
			}
		} else if fn, exists := complexFlags[o]; exists {
			fn(&m)
		} else {
			data = append(data, o)
		}
	}
	m.Data = strings.Join(data, ",")
	if recAttrSet != 0 || recAttrClr != 0 {
		m.RecAttr = &unix.MountAttr{
			Attr_set: recAttrSet,
			Attr_clr: recAttrClr,
		}
	}
	return &m
}

func SetupSeccomp(config *specs.LinuxSeccomp) (*configs.Seccomp, error) {
	if config == nil {
		return nil, nil
	}

	// No default action specified, no syscalls listed, assume seccomp disabled
	if config.DefaultAction == "" && len(config.Syscalls) == 0 {
		return nil, nil
	}

	newConfig := new(configs.Seccomp)
	newConfig.Syscalls = []*configs.Syscall{}

	// The list of flags defined in runtime-spec is a subset of the flags
	// in the seccomp() syscall.
	if config.Flags == nil {
		// No flags are set explicitly (not even the empty set);
		// set the default of specs.LinuxSeccompFlagSpecAllow,
		// if it is supported by the libseccomp and the kernel.
		if err := seccomp.FlagSupported(specs.LinuxSeccompFlagSpecAllow); err == nil {
			newConfig.Flags = []specs.LinuxSeccompFlag{specs.LinuxSeccompFlagSpecAllow}
		}
	} else {
		// Fail early if some flags are unknown or unsupported.
		for _, flag := range config.Flags {
			if err := seccomp.FlagSupported(flag); err != nil {
				return nil, err
			}
			newConfig.Flags = append(newConfig.Flags, flag)
		}
	}

	if len(config.Architectures) > 0 {
		newConfig.Architectures = []string{}
		for _, arch := range config.Architectures {
			newArch, err := seccomp.ConvertStringToArch(string(arch))
			if err != nil {
				return nil, err
			}
			newConfig.Architectures = append(newConfig.Architectures, newArch)
		}
	}

	// Convert default action from string representation
	newDefaultAction, err := seccomp.ConvertStringToAction(string(config.DefaultAction))
	if err != nil {
		return nil, err
	}
	newConfig.DefaultAction = newDefaultAction
	newConfig.DefaultErrnoRet = config.DefaultErrnoRet

	newConfig.ListenerPath = config.ListenerPath
	newConfig.ListenerMetadata = config.ListenerMetadata

	// Loop through all syscall blocks and convert them to libcontainer format
	for _, call := range config.Syscalls {
		newAction, err := seccomp.ConvertStringToAction(string(call.Action))
		if err != nil {
			return nil, err
		}

		for _, name := range call.Names {
			newCall := configs.Syscall{
				Name:     name,
				Action:   newAction,
				ErrnoRet: call.ErrnoRet,
				Args:     []*configs.Arg{},
			}
			// Loop through all the arguments of the syscall and convert them
			for _, arg := range call.Args {
				newOp, err := seccomp.ConvertStringToOperator(string(arg.Op))
				if err != nil {
					return nil, err
				}

				newArg := configs.Arg{
					Index:    arg.Index,
					Value:    arg.Value,
					ValueTwo: arg.ValueTwo,
					Op:       newOp,
				}

				newCall.Args = append(newCall.Args, &newArg)
			}
			newConfig.Syscalls = append(newConfig.Syscalls, &newCall)
		}
	}

	return newConfig, nil
}

func createHooks(rspec *specs.Spec, config *configs.Config) {
	config.Hooks = configs.Hooks{}
	if rspec.Hooks != nil {
		for _, h := range rspec.Hooks.Prestart { //nolint:staticcheck // Ignore SA1019. Need to keep deprecated package for compatibility.
			cmd := createCommandHook(h)
			config.Hooks[configs.Prestart] = append(config.Hooks[configs.Prestart], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.CreateRuntime {
			cmd := createCommandHook(h)
			config.Hooks[configs.CreateRuntime] = append(config.Hooks[configs.CreateRuntime], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.CreateContainer {
			cmd := createCommandHook(h)
			config.Hooks[configs.CreateContainer] = append(config.Hooks[configs.CreateContainer], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.StartContainer {
			cmd := createCommandHook(h)
			config.Hooks[configs.StartContainer] = append(config.Hooks[configs.StartContainer], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.Poststart {
			cmd := createCommandHook(h)
			config.Hooks[configs.Poststart] = append(config.Hooks[configs.Poststart], configs.NewCommandHook(cmd))
		}
		for _, h := range rspec.Hooks.Poststop {
			cmd := createCommandHook(h)
			config.Hooks[configs.Poststop] = append(config.Hooks[configs.Poststop], configs.NewCommandHook(cmd))
		}
	}
}

func createCommandHook(h specs.Hook) *configs.Command {
	cmd := &configs.Command{
		Path: h.Path,
		Args: h.Args,
		Env:  h.Env,
	}
	if h.Timeout != nil {
		d := time.Duration(*h.Timeout) * time.Second
		cmd.Timeout = &d
	}
	return cmd
}

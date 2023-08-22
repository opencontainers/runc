package validate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
	"github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type check func(config *configs.Config) error

func Validate(config *configs.Config) error {
	checks := []check{
		cgroupsCheck,
		rootfs,
		network,
		uts,
		security,
		namespaces,
		sysctl,
		intelrdtCheck,
		rootlessEUIDCheck,
		mountsStrict,
		scheduler,
	}
	for _, c := range checks {
		if err := c(config); err != nil {
			return err
		}
	}
	// Relaxed validation rules for backward compatibility
	warns := []check{
		mountsWarn,
	}
	for _, c := range warns {
		if err := c(config); err != nil {
			logrus.WithError(err).Warn("configuration")
		}
	}
	return nil
}

// rootfs validates if the rootfs is an absolute path and is not a symlink
// to the container's root filesystem.
func rootfs(config *configs.Config) error {
	if _, err := os.Stat(config.Rootfs); err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	cleaned, err := filepath.Abs(config.Rootfs)
	if err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	if cleaned, err = filepath.EvalSymlinks(cleaned); err != nil {
		return fmt.Errorf("invalid rootfs: %w", err)
	}
	if filepath.Clean(config.Rootfs) != cleaned {
		return errors.New("invalid rootfs: not an absolute path, or a symlink")
	}
	return nil
}

func network(config *configs.Config) error {
	if !config.Namespaces.Contains(configs.NEWNET) {
		if len(config.Networks) > 0 || len(config.Routes) > 0 {
			return errors.New("unable to apply network settings without a private NET namespace")
		}
	}
	return nil
}

func uts(config *configs.Config) error {
	if config.Hostname != "" && !config.Namespaces.Contains(configs.NEWUTS) {
		return errors.New("unable to set hostname without a private UTS namespace")
	}
	if config.Domainname != "" && !config.Namespaces.Contains(configs.NEWUTS) {
		return errors.New("unable to set domainname without a private UTS namespace")
	}
	return nil
}

func security(config *configs.Config) error {
	// restrict sys without mount namespace
	if (len(config.MaskPaths) > 0 || len(config.ReadonlyPaths) > 0) &&
		!config.Namespaces.Contains(configs.NEWNS) {
		return errors.New("unable to restrict sys entries without a private MNT namespace")
	}
	if config.ProcessLabel != "" && !selinux.GetEnabled() {
		return errors.New("selinux label is specified in config, but selinux is disabled or not supported")
	}

	return nil
}

func namespaces(config *configs.Config) error {
	if config.Namespaces.Contains(configs.NEWUSER) {
		if _, err := os.Stat("/proc/self/ns/user"); os.IsNotExist(err) {
			return errors.New("USER namespaces aren't enabled in the kernel")
		}
	} else {
		if config.UIDMappings != nil || config.GIDMappings != nil {
			return errors.New("User namespace mappings specified, but USER namespace isn't enabled in the config")
		}
	}

	if config.Namespaces.Contains(configs.NEWCGROUP) {
		if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
			return errors.New("cgroup namespaces aren't enabled in the kernel")
		}
	}

	if config.Namespaces.Contains(configs.NEWTIME) {
		if _, err := os.Stat("/proc/self/timens_offsets"); os.IsNotExist(err) {
			return errors.New("time namespaces aren't enabled in the kernel")
		}
	} else {
		if config.TimeOffsets != nil {
			return errors.New("time namespace offsets specified, but time namespace isn't enabled in the config")
		}
	}

	return nil
}

// convertSysctlVariableToDotsSeparator can return sysctl variables in dots separator format.
// The '/' separator is also accepted in place of a '.'.
// Convert the sysctl variables to dots separator format for validation.
// More info: sysctl(8), sysctl.d(5).
//
// For example:
// Input sysctl variable "net/ipv4/conf/eno2.100.rp_filter"
// will return the converted value "net.ipv4.conf.eno2/100.rp_filter"
func convertSysctlVariableToDotsSeparator(val string) string {
	if val == "" {
		return val
	}
	firstSepIndex := strings.IndexAny(val, "./")
	if firstSepIndex == -1 || val[firstSepIndex] == '.' {
		return val
	}

	f := func(r rune) rune {
		switch r {
		case '.':
			return '/'
		case '/':
			return '.'
		}
		return r
	}
	return strings.Map(f, val)
}

// sysctl validates that the specified sysctl keys are valid or not.
// /proc/sys isn't completely namespaced and depending on which namespaces
// are specified, a subset of sysctls are permitted.
func sysctl(config *configs.Config) error {
	validSysctlMap := map[string]bool{
		"kernel.msgmax":          true,
		"kernel.msgmnb":          true,
		"kernel.msgmni":          true,
		"kernel.sem":             true,
		"kernel.shmall":          true,
		"kernel.shmmax":          true,
		"kernel.shmmni":          true,
		"kernel.shm_rmid_forced": true,
	}

	var (
		netOnce    sync.Once
		hostnet    bool
		hostnetErr error
	)

	for s := range config.Sysctl {
		s := convertSysctlVariableToDotsSeparator(s)
		if validSysctlMap[s] || strings.HasPrefix(s, "fs.mqueue.") {
			if config.Namespaces.Contains(configs.NEWIPC) {
				continue
			} else {
				return fmt.Errorf("sysctl %q is not allowed in the hosts ipc namespace", s)
			}
		}
		if strings.HasPrefix(s, "net.") {
			// Is container using host netns?
			// Here "host" means "current", not "initial".
			netOnce.Do(func() {
				if !config.Namespaces.Contains(configs.NEWNET) {
					hostnet = true
					return
				}
				path := config.Namespaces.PathOf(configs.NEWNET)
				if path == "" {
					// own netns, so hostnet = false
					return
				}
				hostnet, hostnetErr = isHostNetNS(path)
			})
			if hostnetErr != nil {
				return fmt.Errorf("invalid netns path: %w", hostnetErr)
			}
			if hostnet {
				return fmt.Errorf("sysctl %q not allowed in host network namespace", s)
			}
			continue
		}
		if config.Namespaces.Contains(configs.NEWUTS) {
			switch s {
			case "kernel.domainname":
				// This is namespaced and there's no explicit OCI field for it.
				continue
			case "kernel.hostname":
				// This is namespaced but there's a conflicting (dedicated) OCI field for it.
				return fmt.Errorf("sysctl %q is not allowed as it conflicts with the OCI %q field", s, "hostname")
			}
		}
		return fmt.Errorf("sysctl %q is not in a separate kernel namespace", s)
	}

	return nil
}

func intelrdtCheck(config *configs.Config) error {
	if config.IntelRdt != nil {
		if config.IntelRdt.ClosID == "." || config.IntelRdt.ClosID == ".." || strings.Contains(config.IntelRdt.ClosID, "/") {
			return fmt.Errorf("invalid intelRdt.ClosID %q", config.IntelRdt.ClosID)
		}

		if !intelrdt.IsCATEnabled() && config.IntelRdt.L3CacheSchema != "" {
			return errors.New("intelRdt.l3CacheSchema is specified in config, but Intel RDT/CAT is not enabled")
		}
		if !intelrdt.IsMBAEnabled() && config.IntelRdt.MemBwSchema != "" {
			return errors.New("intelRdt.memBwSchema is specified in config, but Intel RDT/MBA is not enabled")
		}
	}

	return nil
}

func cgroupsCheck(config *configs.Config) error {
	c := config.Cgroups
	if c == nil {
		return nil
	}

	if (c.Name != "" || c.Parent != "") && c.Path != "" {
		return fmt.Errorf("cgroup: either Path or Name and Parent should be used, got %+v", c)
	}

	r := c.Resources
	if r == nil {
		return nil
	}

	if !cgroups.IsCgroup2UnifiedMode() && r.Unified != nil {
		return cgroups.ErrV1NoUnified
	}

	if cgroups.IsCgroup2UnifiedMode() {
		_, err := cgroups.ConvertMemorySwapToCgroupV2Value(r.MemorySwap, r.Memory)
		if err != nil {
			return err
		}
	}

	return nil
}

func checkBindOptions(m *configs.Mount) error {
	if !m.IsBind() {
		return nil
	}
	// We must reject bind-mounts that also have filesystem-specific mount
	// options, because the kernel will completely ignore these flags and we
	// cannot set them per-mountpoint.
	//
	// It should be noted that (due to how the kernel caches superblocks), data
	// options could also silently ignored for other filesystems even when
	// doing a fresh mount, but there is no real way to avoid this (and it
	// matches how everything else works). There have been proposals to make it
	// possible for userspace to detect this caching, but this wouldn't help
	// runc because the behaviour wouldn't even be desirable for most users.
	if m.Data != "" {
		return errors.New("bind mounts cannot have any filesystem-specific options applied")
	}
	return nil
}

func checkIDMapMounts(config *configs.Config, m *configs.Mount) error {
	if !m.IsIDMapped() {
		return nil
	}

	if !m.IsBind() {
		return fmt.Errorf("gidMappings/uidMappings is supported only for mounts with the option 'bind'")
	}
	if config.RootlessEUID {
		return fmt.Errorf("gidMappings/uidMappings is not supported when runc is being launched with EUID != 0, needs CAP_SYS_ADMIN on the runc parent's user namespace")
	}
	if len(config.UIDMappings) == 0 || len(config.GIDMappings) == 0 {
		return fmt.Errorf("not yet supported to use gidMappings/uidMappings in a mount without also using a user namespace")
	}
	if !sameMapping(config.UIDMappings, m.UIDMappings) {
		return fmt.Errorf("not yet supported for the mount uidMappings to be different than user namespace uidMapping")
	}
	if !sameMapping(config.GIDMappings, m.GIDMappings) {
		return fmt.Errorf("not yet supported for the mount gidMappings to be different than user namespace gidMapping")
	}
	if !filepath.IsAbs(m.Source) {
		return fmt.Errorf("mount source not absolute")
	}

	return nil
}

func mountsWarn(config *configs.Config) error {
	for _, m := range config.Mounts {
		if !filepath.IsAbs(m.Destination) {
			return fmt.Errorf("mount %+v: relative destination path is **deprecated**, using it as relative to /", m)
		}
	}
	return nil
}

func mountsStrict(config *configs.Config) error {
	for _, m := range config.Mounts {
		if err := checkBindOptions(m); err != nil {
			return fmt.Errorf("invalid mount %+v: %w", m, err)
		}
		if err := checkIDMapMounts(config, m); err != nil {
			return fmt.Errorf("invalid mount %+v: %w", m, err)
		}
	}
	return nil
}

// sameMapping checks if the mappings are the same. If the mappings are the same
// but in different order, it returns false.
func sameMapping(a, b []configs.IDMap) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].ContainerID != b[i].ContainerID {
			return false
		}
		if a[i].HostID != b[i].HostID {
			return false
		}
		if a[i].Size != b[i].Size {
			return false
		}
	}
	return true
}

func isHostNetNS(path string) (bool, error) {
	const currentProcessNetns = "/proc/self/ns/net"

	var st1, st2 unix.Stat_t

	if err := unix.Stat(currentProcessNetns, &st1); err != nil {
		return false, &os.PathError{Op: "stat", Path: currentProcessNetns, Err: err}
	}
	if err := unix.Stat(path, &st2); err != nil {
		return false, &os.PathError{Op: "stat", Path: path, Err: err}
	}

	return (st1.Dev == st2.Dev) && (st1.Ino == st2.Ino), nil
}

// scheduler is to validate scheduler configs according to https://man7.org/linux/man-pages/man2/sched_setattr.2.html
func scheduler(config *configs.Config) error {
	s := config.Scheduler
	if s == nil {
		return nil
	}
	if s.Policy == "" {
		return errors.New("scheduler policy is required")
	}
	if s.Nice < -20 || s.Nice > 19 {
		return fmt.Errorf("invalid scheduler.nice: %d", s.Nice)
	}
	if s.Priority != 0 && (s.Policy != specs.SchedFIFO && s.Policy != specs.SchedRR) {
		return errors.New("scheduler.priority can only be specified for SchedFIFO or SchedRR policy")
	}
	if s.Policy != specs.SchedDeadline && (s.Runtime != 0 || s.Deadline != 0 || s.Period != 0) {
		return errors.New("scheduler runtime/deadline/period can only be specified for SchedDeadline policy")
	}
	return nil
}

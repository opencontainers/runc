package configs

import "fmt"

type Rlimit struct {
	Type int    `json:"type,omitempty"`
	Hard uint64 `json:"hard,omitempty"`
	Soft uint64 `json:"soft,omitempty"`
}

// IDMap represents UID/GID Mappings for User Namespaces.
type IDMap struct {
	ContainerID int `json:"container_id,omitempty"`
	HostID      int `json:"host_id,omitempty"`
	Size        int `json:"size,omitempty"`
}

// Config defines configuration options for executing a process inside a contained environment.
type Config struct {
	// NoPivotRoot will use MS_MOVE and a chroot to jail the process into the container's rootfs
	// This is a common option when the container is running in ramdisk
	NoPivotRoot bool `json:"no_pivot_root,omitempty"`

	// ParentDeathSignal specifies the signal that is sent to the container's process in the case
	// that the parent process dies.
	ParentDeathSignal int `json:"parent_death_signal,omitempty"`

	// PivotDir allows a custom directory inside the container's root filesystem to be used as pivot, when NoPivotRoot is not set.
	// When a custom PivotDir not set, a temporary dir inside the root filesystem will be used. The pivot dir needs to be writeable.
	// This is required when using read only root filesystems. In these cases, a read/writeable path can be (bind) mounted somewhere inside the root filesystem to act as pivot.
	PivotDir string `json:"pivot_dir,omitempty"`

	// Path to a directory containing the container's root filesystem.
	Rootfs string `json:"rootfs,omitempty"`

	// Readonlyfs will remount the container's rootfs as readonly where only externally mounted
	// bind mounts are writtable.
	Readonlyfs bool `json:"readonlyfs,omitempty"`

	// Mounts specify additional source and destination paths that will be mounted inside the container's
	// rootfs and mount namespace if specified
	Mounts []*Mount `json:"mounts,omitempty"`

	// The device nodes that should be automatically created within the container upon container start.  Note, make sure that the node is marked as allowed in the cgroup as well!
	Devices []*Device `json:"devices,omitempty"`

	MountLabel string `json:"mount_label,omitempty"`

	// Hostname optionally sets the container's hostname if provided
	Hostname string `json:"hostname,omitempty"`

	// User will set the uid and gid of the executing process running inside the container
	User string `json:"user,omitempty"`

	// WorkingDir will change the processes current working directory inside the container's rootfs
	WorkingDir string `json:"working_dir,omitempty"`

	// Env will populate the processes environment with the provided values
	// Any values from the parent processes will be cleared before the values
	// provided in Env are provided to the process
	Env []string `json:"environment,omitempty"`

	// Console is the path to the console allocated to the container.
	Console string `json:"console,omitempty"`

	// Namespaces specifies the container's namespaces that it should setup when cloning the init process
	// If a namespace is not provided that namespace is shared from the container's parent process
	Namespaces Namespaces `json:"namespaces,omitempty"`

	// Capabilities specify the capabilities to keep when executing the process inside the container
	// All capbilities not specified will be dropped from the processes capability mask
	Capabilities []string `json:"capabilities,omitempty"`

	// Networks specifies the container's network setup to be created
	Networks []*Network `json:"networks,omitempty"`

	// Routes can be specified to create entries in the route table as the container is started
	Routes []*Route `json:"routes,omitempty"`

	// Cgroups specifies specific cgroup settings for the various subsystems that the container is
	// placed into to limit the resources the container has available
	Cgroups *Cgroup `json:"cgroups,omitempty"`

	// AppArmorProfile specifies the profile to apply to the process running in the container and is
	// change at the time the process is execed
	AppArmorProfile string `json:"apparmor_profile,omitempty"`

	// ProcessLabel specifies the label to apply to the process running in the container.  It is
	// commonly used by selinux
	ProcessLabel string `json:"process_label,omitempty"`

	// RestrictSys will remount /proc/sys, /sys, and mask over sysrq-trigger as well as /proc/irq and
	// /proc/bus
	RestrictSys bool `json:"restrict_sys,omitempty"`

	// Rlimits specifies the resource limits, such as max open files, to set in the container
	// If Rlimits are not set, the container will inherit rlimits from the parent process
	Rlimits []Rlimit `json:"rlimits,omitempty"`

	// AdditionalGroups specifies the gids that should be added to supplementary groups
	// in addition to those that the user belongs to.
	AdditionalGroups []int `json:"additional_groups,omitempty"`

	// UidMappings is an array of User ID mappings for User Namespaces
	UidMappings []IDMap `json:"uid_mappings,omitempty"`

	// GidMappings is an array of Group ID mappings for User Namespaces
	GidMappings []IDMap `json:"gid_mappings,omitempty"`
}

// Gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled.
func (c *Config) HostUID() (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if c.UidMappings == nil {
			return -1, fmt.Errorf("User namespaces enabled, but no user mappings found.")
		}
		id, found := c.hostIDFromMapping(0, c.UidMappings)
		if !found {
			return -1, fmt.Errorf("User namespaces enabled, but no root user mapping found.")
		}
		return id, nil
	}
	// Return default root uid 0
	return 0, nil
}

// Gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled.
func (c *Config) HostGID() (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if c.GidMappings == nil {
			return -1, fmt.Errorf("User namespaces enabled, but no gid mappings found.")
		}
		id, found := c.hostIDFromMapping(0, c.GidMappings)
		if !found {
			return -1, fmt.Errorf("User namespaces enabled, but no root user mapping found.")
		}
		return id, nil
	}
	// Return default root uid 0
	return 0, nil
}

// Utility function that gets a host ID for a container ID from user namespace map
// if that ID is present in the map.
func (c *Config) hostIDFromMapping(containerID int, uMap []IDMap) (int, bool) {
	for _, m := range uMap {
		if (containerID >= m.ContainerID) && (containerID <= (m.ContainerID + m.Size - 1)) {
			hostID := m.HostID + (containerID - m.ContainerID)
			return hostID, true
		}
	}
	return -1, false
}

package configs

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

var (
	errNoUIDMap   = errors.New("User namespaces enabled, but no uid mappings found.")
	errNoUserMap  = errors.New("User namespaces enabled, but no user mapping found.")
	errNoGIDMap   = errors.New("User namespaces enabled, but no gid mappings found.")
	errNoGroupMap = errors.New("User namespaces enabled, but no group mapping found.")
)

// HostUID gets the translated uid for the process on host which could be
// different when user namespaces are enabled.
func (c Config) HostUID(containerId int) (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if c.UidMappings == nil {
			return -1, errNoUIDMap
		}
		id, found := c.hostIDFromMapping(containerId, c.UidMappings)
		if !found {
			return -1, errNoUserMap
		}
		return id, nil
	}
	// Return unchanged id.
	return containerId, nil
}

// HostRootUID gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled.
func (c Config) HostRootUID() (int, error) {
	return c.HostUID(0)
}

// HostGID gets the translated gid for the process on host which could be
// different when user namespaces are enabled.
func (c Config) HostGID(containerId int) (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if c.GidMappings == nil {
			return -1, errNoGIDMap
		}
		id, found := c.hostIDFromMapping(containerId, c.GidMappings)
		if !found {
			return -1, errNoGroupMap
		}
		return id, nil
	}
	// Return unchanged id.
	return containerId, nil
}

// HostRootGID gets the root gid for the process on host which could be non-zero
// when user namespaces are enabled.
func (c Config) HostRootGID() (int, error) {
	return c.HostGID(0)
}

// Utility function that gets a host ID for a container ID from user namespace map
// if that ID is present in the map.
func (c Config) hostIDFromMapping(containerID int, uMap []IDMap) (int, bool) {
	for _, m := range uMap {
		if (containerID >= m.ContainerID) && (containerID <= (m.ContainerID + m.Size - 1)) {
			hostID := m.HostID + (containerID - m.ContainerID)
			return hostID, true
		}
	}
	return -1, false
}

// IsHostNetNS returns true if this container is configured to run in the same
// net namespace as the host
func (c Config) IsHostNetNS() (bool, error) {
	if !c.Namespaces.Contains(NEWNET) {
		return true, nil
	}

	path := c.Namespaces.PathOf(NEWNET)
	if path == "" {
		// own netns, so hostnet = false
		return false, nil
	}

	return isHostNetNS(path)
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

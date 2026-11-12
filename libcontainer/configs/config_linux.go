package configs

import (
	"errors"
	"fmt"
	"math"
)

var (
	errNoUIDMap = errors.New("user namespaces enabled, but no uid mappings found")
	errNoGIDMap = errors.New("user namespaces enabled, but no gid mappings found")
)

// Please check https://man7.org/linux/man-pages/man2/personality.2.html for const details.
// https://raw.githubusercontent.com/torvalds/linux/master/include/uapi/linux/personality.h
const (
	PerLinux   = 0x0000
	PerLinux32 = 0x0008
)

type LinuxPersonality struct {
	// Domain for the personality
	// can only contain values "LINUX" and "LINUX32"
	Domain int `json:"domain"`
}

// HostUID gets the translated uid for the process on host which could be
// different when user namespaces are enabled.
func (c *Config) HostUID(containerID int) (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if len(c.UIDMappings) == 0 {
			return -1, errNoUIDMap
		}
		id, found := c.hostIDFromMapping(int64(containerID), c.UIDMappings)
		if !found {
			return -1, fmt.Errorf("user namespaces enabled, but no mapping found for uid %d", containerID)
		}
		// If we are a 32-bit binary running on a 64-bit system, it's possible
		// the mapped user is too large to store in an int, which means we
		// cannot do the mapping. We can't just return an int64, because
		// os.Setuid() takes an int.
		if id > math.MaxInt {
			return -1, fmt.Errorf("mapping for uid %d (host id %d) is larger than native integer size (%d)", containerID, id, math.MaxInt)
		}
		return int(id), nil
	}
	// Return unchanged id.
	return containerID, nil
}

// HostRootUID gets the root uid for the process on host which could be non-zero
// when user namespaces are enabled. If user namespaces are enabled but uid 0
// is not mapped, the host uid of the lowest mapped container uid is returned
// (which is what runc init uses to set up the container in such case).
func (c *Config) HostRootUID() (int, error) {
	return c.HostUID(c.rootID(c.UIDMappings))
}

// HostGID gets the translated gid for the process on host which could be
// different when user namespaces are enabled.
func (c *Config) HostGID(containerID int) (int, error) {
	if c.Namespaces.Contains(NEWUSER) {
		if len(c.GIDMappings) == 0 {
			return -1, errNoGIDMap
		}
		id, found := c.hostIDFromMapping(int64(containerID), c.GIDMappings)
		if !found {
			return -1, fmt.Errorf("user namespaces enabled, but no mapping found for gid %d", containerID)
		}
		// If we are a 32-bit binary running on a 64-bit system, it's possible
		// the mapped user is too large to store in an int, which means we
		// cannot do the mapping. We can't just return an int64, because
		// os.Setgid() takes an int.
		if id > math.MaxInt {
			return -1, fmt.Errorf("mapping for gid %d (host id %d) is larger than native integer size (%d)", containerID, id, math.MaxInt)
		}
		return int(id), nil
	}
	// Return unchanged id.
	return containerID, nil
}

// HostRootGID gets the root gid for the process on host which could be non-zero
// when user namespaces are enabled. If user namespaces are enabled but gid 0
// is not mapped, the host gid of the lowest mapped container gid is returned
// (which is what runc init uses to set up the container in such case).
func (c *Config) HostRootGID() (int, error) {
	return c.HostGID(c.rootID(c.GIDMappings))
}

// UsernsRootIDs returns the container uid and gid which runc init uses as root
// to set up the container. Both are 0, unless user namespaces are enabled and 0
// is not mapped, in which case the lowest mapped container ID is used.
func (c *Config) UsernsRootIDs() (uid, gid int) {
	return c.rootID(c.UIDMappings), c.rootID(c.GIDMappings)
}

// rootID returns the container ID which runc init uses as root to set up
// the container. This is 0, unless user namespaces are enabled and 0 is not
// mapped, in which case it is the lowest mapped container ID.
func (c *Config) rootID(idMap []IDMap) int {
	if !c.Namespaces.Contains(NEWUSER) || len(idMap) == 0 {
		return 0
	}
	if _, found := c.hostIDFromMapping(0, idMap); found {
		return 0
	}
	minID := idMap[0].ContainerID
	for _, m := range idMap[1:] {
		minID = min(minID, m.ContainerID)
	}
	return int(minID)
}

// Utility function that gets a host ID for a container ID from user namespace map
// if that ID is present in the map.
func (c *Config) hostIDFromMapping(containerID int64, uMap []IDMap) (int64, bool) {
	for _, m := range uMap {
		if (containerID >= m.ContainerID) && (containerID <= (m.ContainerID + m.Size - 1)) {
			hostID := m.HostID + (containerID - m.ContainerID)
			return hostID, true
		}
	}
	return -1, false
}

//go:build linux

package userns

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func parseIdmapData(data []byte) (ms []configs.IDMap, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		var m configs.IDMap
		line := scanner.Text()
		if _, err := fmt.Sscanf(line, "%d %d %d", &m.ContainerID, &m.HostID, &m.Size); err != nil {
			return nil, fmt.Errorf("parsing id map failed: invalid format in line %q: %w", line, err)
		}
		ms = append(ms, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parsing id map failed: %w", err)
	}
	return ms, nil
}

func GetUserNamespaceMappings(nsPath string) (uidMap, gidMap []configs.IDMap, err error) {
	var (
		pid         int
		extra       rune
		tryFastPath bool
	)

	// nsPath is usually of the form /proc/<pid>/ns/user, which means that we
	// already have a pid that is part of the user namespace and thus we can
	// just use the pid to read from /proc/<pid>/*id_map.
	//
	// Note that Sscanf doesn't consume the whole input, so we check for any
	// trailing data with %c. That way, we can be sure the pattern matched
	// /proc/$pid/ns/user _exactly_ iff n === 1.
	if n, _ := fmt.Sscanf(nsPath, "/proc/%d/ns/user%c", &pid, &extra); n == 1 {
		tryFastPath = pid > 0
	}

	for _, mapType := range []struct {
		name  string
		idMap *[]configs.IDMap
	}{
		{"uid_map", &uidMap},
		{"gid_map", &gidMap},
	} {
		var mapData []byte

		if tryFastPath {
			path := fmt.Sprintf("/proc/%d/%s", pid, mapType.name)
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			mapData = data
		} else {
			return nil, nil, errors.New("userns-cat missing")
		}
		idMap, err := parseIdmapData(mapData)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s of userns %s: %w", mapType.name, nsPath, err)
		}
		*mapType.idMap = idMap
	}

	return uidMap, gidMap, nil
}

// IsSameMapping returns whether or not the two id mappings are the same. Note
// that if the order of the mappings is different, or a mapping has been split,
// the mappings will be considered different.
func IsSameMapping(a, b []configs.IDMap) bool {
	if len(a) != len(b) {
		return false
	}
	for idx := range a {
		if a[idx] != b[idx] {
			return false
		}
	}
	return true
}

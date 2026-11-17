//go:build linux

package userns

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/internal/rawfork"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
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

// Do something equivalent to nsenter --user=<nsPath> cat <path>, but more
// efficiently. Returns the contents of the requested file from within the user
// namespace.
func spawnUserNamespaceCat(nsPath, path string) ([]byte, error) {
	nsFile, err := os.Open(nsPath)
	if err != nil {
		return nil, fmt.Errorf("open userns path failed: %w", err)
	}
	defer nsFile.Close()

	rdr, wtr, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create pipe for userns spawn failed: %w", err)
	}
	defer rdr.Close()
	defer wtr.Close()

	// The child joins the user namespace, opens the file, and copies its
	// contents to the pipe. Since the pipe is blocking and the child has no
	// signal handlers, writes to it are never short.
	var prog rawfork.Program
	pathPtr, err := prog.String(path)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 4096)
	bufPtr := prog.Bytes(buf)
	fdcwd := unix.AT_FDCWD
	prog.Ops = []rawfork.Op{
		{
			Name: "setns " + nsPath,
			Trap: unix.SYS_SETNS,
			Args: [6]uintptr{nsFile.Fd(), unix.CLONE_NEWUSER},
		},
		{
			Name: "open " + path,
			Trap: unix.SYS_OPENAT,
			Args: [6]uintptr{uintptr(fdcwd), pathPtr, unix.O_RDONLY | unix.O_CLOEXEC},
			Ret:  rawfork.R1,
		},
		{ // 2
			Name:    "read " + path,
			Trap:    unix.SYS_READ,
			Args:    [6]uintptr{0, bufPtr, uintptr(len(buf))},
			ArgRegs: [6]rawfork.Reg{rawfork.R1},
			Ret:     rawfork.R2,
			Cond:    rawfork.IfZero,
			Goto:    4, // EOF, exit.
		},
		{
			Name:    "write to pipe",
			Trap:    unix.SYS_WRITE,
			Args:    [6]uintptr{wtr.Fd(), bufPtr},
			ArgRegs: [6]rawfork.Reg{2: rawfork.R2},
			Cond:    rawfork.Always,
			Goto:    2, // Read more.
		},
	}
	child, err := prog.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to spawn fork for userns: %w", err)
	}

	// We are in the parent -- close the write end of the pipe before reading.
	wtr.Close()
	output, readErr := io.ReadAll(rdr)
	if err := child.Wait(); err != nil {
		return nil, fmt.Errorf("userns spawn: %w", err)
	}
	if readErr != nil {
		return nil, fmt.Errorf("reading from userns spawn failed: %w", readErr)
	}
	return output, nil
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
				// Do not error out here -- we need to try the slow path if the
				// fast path failed.
				logrus.Debugf("failed to use fast path to read %s from userns %s (error: %s), falling back to slow userns-join path", mapType.name, nsPath, err)
			} else {
				mapData = data
			}
		} else {
			logrus.Debugf("cannot use fast path to read %s from userns %s, falling back to slow userns-join path", mapType.name, nsPath)
		}

		if mapData == nil {
			// We have to actually join the namespace if we cannot take the
			// fast path. The path is resolved with respect to the child
			// process, so just use /proc/self.
			data, err := spawnUserNamespaceCat(nsPath, "/proc/self/"+mapType.name)
			if err != nil {
				return nil, nil, err
			}
			mapData = data
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

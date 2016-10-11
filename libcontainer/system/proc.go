// +build linux

package system

import (
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

// GetProcessStartTime reads /proc/<pid>/stat to determine the "start time" of
// a process as provided by the kernel. The format of this start time is
// defined by proc(5) and is not parsed or otherwise handled by us. Combined
// with the process ID, this function allows for verification of whether or not
// a particular PID is the same process at two different points in time.
func GetProcessStartTime(pid int) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", err
	}

	parts := strings.Split(string(data), " ")
	// the starttime is located at pos 22
	// from the man page
	//
	// starttime %llu (was %lu before Linux 2.6)
	// (22)  The  time the process started after system boot.  In kernels before Linux 2.6, this
	// value was expressed in jiffies.  Since Linux 2.6, the value is expressed in  clock  ticks
	// (divide by sysconf(_SC_CLK_TCK)).
	return parts[22-1], nil // starts at 1
}

// GetProcessState reads /proc/<pid>/stat to determine the "state" of a process
// as provided by the kernel. The format of the state is defined by proc(5) and
// is not parsed by us, though it is usually a single character mnemonic.
func GetProcessState(pid int) (string, error) {
	data, err := ioutil.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return "", err
	}

	parts := strings.Split(string(data), " ")
	// the state field is located in pos 3
	return parts[3-1], nil // starts at 1
}

// GetProcessParent reads reads /proc/<pid>/stat to determine the parent of a
// process.
func GetProcessParent(pid int) (int, error) {
	data, err := ioutil.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return -1, err
	}

	parts := strings.Split(string(data), " ")
	// the state field is located in pos 4
	return strconv.Atoi(parts[4-1]) // starts at 1
}

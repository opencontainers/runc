package system

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
)

// Stat_t represents the information from /proc/[pid]/stat, as
// described in proc(5).
type Stat_t struct {
	// StartTime is the number of clock ticks after system boot (since
	// Linux 2.6).
	StartTime uint64
}

// Stat returns a Stat_t instance for the specified process.
func Stat(pid int) (stat Stat_t, err error) {
	bytes, err := ioutil.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return stat, err
	}
	data := string(bytes)
	stat.StartTime, err = parseStartTime(data)
	return stat, err
}

// GetProcessStartTime is deprecated.  Use Stat(pid) and
// Stat_t.StartTime instead.
func GetProcessStartTime(pid int) (string, error) {
	stat, err := Stat(pid)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d", stat.StartTime), nil
}

func parseStartTime(stat string) (uint64, error) {
	// the starttime is located at pos 22
	// from the man page
	//
	// starttime %llu (was %lu before Linux 2.6)
	// (22)  The  time the process started after system boot.  In kernels before Linux 2.6, this
	// value was expressed in jiffies.  Since Linux 2.6, the value is expressed in  clock  ticks
	// (divide by sysconf(_SC_CLK_TCK)).
	//
	// NOTE:
	// pos 2 could contain space and is inside `(` and `)`:
	// (2) comm  %s
	// The filename of the executable, in parentheses.
	// This is visible whether or not the executable is
	// swapped out.
	//
	// the following is an example:
	// 89653 (gunicorn: maste) S 89630 89653 89653 0 -1 4194560 29689 28896 0 3 146 32 76 19 20 0 1 0 2971844 52965376 3920 18446744073709551615 1 1 0 0 0 0 0 16781312 137447943 0 0 0 17 1 0 0 0 0 0 0 0 0 0 0 0 0 0

	// get parts after last `)`:
	s := strings.Split(stat, ")")
	parts := strings.Split(strings.TrimSpace(s[len(s)-1]), " ")
	startTimeString := parts[22-3] // starts at 3 (after the filename pos `2`)
	var startTime uint64
	fmt.Sscanf(startTimeString, "%d", &startTime)
	return startTime, nil
}

package system

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/internal/pathrs"
)

// State is the status of a process.
type State rune

const ( // Only values for Linux 3.14 and later are listed here
	Dead        State = 'X'
	DiskSleep   State = 'D'
	Running     State = 'R'
	Sleeping    State = 'S'
	Stopped     State = 'T'
	TracingStop State = 't'
	Zombie      State = 'Z'
	Parked      State = 'P'
	Idle        State = 'I'
)

// String forms of the state from proc(5)'s documentation for
// /proc/[pid]/status' "State" field.
func (s State) String() string {
	switch s {
	case Dead:
		return "dead"
	case DiskSleep:
		return "disk sleep"
	case Running:
		return "running"
	case Sleeping:
		return "sleeping"
	case Stopped:
		return "stopped"
	case TracingStop:
		return "tracing stop"
	case Zombie:
		return "zombie"
	case Parked:
		return "parked"
	case Idle:
		return "idle" // kernel thread
	default:
		return fmt.Sprintf("unknown (%c)", s)
	}
}

// Stat_t represents the information from /proc/[pid]/stat, as
// described in proc(5) with names based on the /proc/[pid]/status
// fields.
type Stat_t struct {
	// Name is the command run by the process.
	Name string

	// State is the state of the process.
	State State

	// NumThreads is the number of threads in the process.
	NumThreads int64

	// StartTime is the number of clock ticks after system boot (since
	// Linux 2.6).
	StartTime uint64
}

// Stat returns a Stat_t instance for the specified process.
func Stat(pid int) (Stat_t, error) {
	var stat Stat_t

	statFile, err := pathrs.ProcPidOpen(pid, "stat", os.O_RDONLY)
	if err != nil {
		return stat, err
	}
	defer statFile.Close()

	bytes, err := io.ReadAll(statFile)
	if err != nil {
		return stat, err
	}
	return parseStat(string(bytes))
}

func parseStat(data string) (stat Stat_t, err error) {
	// Example:
	// 89653 (gunicorn: maste) S 89630 89653 89653 0 -1 4194560 29689 28896 0 3 146 32 76 19 20 0 1 0 2971844 52965376 3920 18446744073709551615 1 1 0 0 0 0 0 16781312 137447943 0 0 0 17 1 0 0 0 0 0 0 0 0 0 0 0 0 0
	// The fields are space-separated, see full description in proc(5).
	//
	// We are only interested in:
	//  * field 2: process name. It is the only field enclosed into
	//    parenthesis, as it can contain spaces (and parenthesis) inside.
	//  * field 3: process state, a single character (%c)
	//  * field 20: number of threads, a long integer (%ld)
	//  * field 22: process start time, a long unsigned integer (%llu).

	// 1. Look for the first '(' and the last ')' first, what's in between is Name.
	//    We expect at least 20 fields and a space after the last one.

	const minAfterName = 20*2 + 1 // the min field is '0 '.

	first := strings.IndexByte(data, '(')
	if first < 0 || first+minAfterName >= len(data) {
		return stat, fmt.Errorf("invalid stat data (no comm or too short): %q", data)
	}

	last := strings.LastIndexByte(data, ')')
	if last <= first || last+minAfterName >= len(data) {
		return stat, fmt.Errorf("invalid stat data (no comm or too short): %q", data)
	}

	stat.Name = data[first+1 : last]

	// 2. Remove fields 1 and 2 and a space after. State is right after.
	data = data[last+2:]
	stat.State = State(data[0])

	// 3. Split the rest into fields; data is at field 3 now. The last
	//    element is whatever is after StartTime (field 22) and a space.
	const fieldsNeeded = 22 - 3 + 2
	fields := strings.SplitN(data, " ", fieldsNeeded)
	if len(fields) < fieldsNeeded {
		return stat, fmt.Errorf("invalid stat data (too short): %q", data)
	}
	stat.NumThreads, err = strconv.ParseInt(fields[20-3], 10, 64)
	if err != nil {
		return stat, fmt.Errorf("invalid stat data (bad number of threads): %w", err)
	}
	stat.StartTime, err = strconv.ParseUint(fields[22-3], 10, 64)
	if err != nil {
		return stat, fmt.Errorf("invalid stat data (bad start time): %w", err)
	}

	return stat, nil
}

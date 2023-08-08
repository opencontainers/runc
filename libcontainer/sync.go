package libcontainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/opencontainers/runc/libcontainer/utils"
)

type syncType string

// Constants that are used for synchronisation between the parent and child
// during container setup. They come in pairs (with procError being a generic
// response which is followed by an &initError).
//
//	     [  child  ] <-> [   parent   ]
//
//	procSeccomp         --> [forward fd to listenerPath]
//	  file: seccomp fd
//	                    --- no return synchronisation
//
//	procHooks --> [run hooks]
//	          <-- procResume
//
//	procReady --> [final setup]
//	          <-- procRun
//
//	procSeccomp --> [grab seccomp fd with pidfd_getfd()]
//	            <-- procSeccompDone
const (
	procError       syncType = "procError"
	procReady       syncType = "procReady"
	procRun         syncType = "procRun"
	procHooks       syncType = "procHooks"
	procResume      syncType = "procResume"
	procSeccomp     syncType = "procSeccomp"
	procSeccompDone syncType = "procSeccompDone"
)

type syncFlags int

const (
	syncFlagHasFd syncFlags = (1 << iota)
)

type syncT struct {
	Type  syncType         `json:"type"`
	Flags syncFlags        `json:"flags"`
	Arg   *json.RawMessage `json:"arg,omitempty"`
	File  *os.File         `json:"-"` // passed oob through SCM_RIGHTS
}

// initError is used to wrap errors for passing them via JSON,
// as encoding/json can't unmarshal into error type.
type initError struct {
	Message string `json:"message,omitempty"`
}

func (i initError) Error() string {
	return i.Message
}

func doWriteSync(pipe *os.File, sync syncT) error {
	sync.Flags &= ^syncFlagHasFd
	if sync.File != nil {
		sync.Flags |= syncFlagHasFd
	}
	if err := utils.WriteJSON(pipe, sync); err != nil {
		return fmt.Errorf("writing sync %q: %w", sync.Type, err)
	}
	if sync.Flags&syncFlagHasFd != 0 {
		if err := utils.SendFile(pipe, sync.File); err != nil {
			return fmt.Errorf("sending file after sync %q: %w", sync.Type, err)
		}
	}
	return nil
}

func writeSync(pipe *os.File, sync syncType) error {
	return doWriteSync(pipe, syncT{Type: sync})
}

func writeSyncArg(pipe *os.File, sync syncType, arg interface{}) error {
	argJSON, err := json.Marshal(arg)
	if err != nil {
		return fmt.Errorf("writing sync %q: marshal argument failed: %w", sync, err)
	}
	argJSONMsg := json.RawMessage(argJSON)
	return doWriteSync(pipe, syncT{Type: sync, Arg: &argJSONMsg})
}

func doReadSync(pipe *os.File) (syncT, error) {
	var sync syncT
	if err := json.NewDecoder(pipe).Decode(&sync); err != nil {
		if errors.Is(err, io.EOF) {
			return sync, err
		}
		return sync, fmt.Errorf("reading from parent failed: %w", err)
	}
	if sync.Type == procError {
		var ierr initError
		if sync.Arg == nil {
			return sync, errors.New("procError missing error payload")
		}
		if err := json.Unmarshal(*sync.Arg, &ierr); err != nil {
			return sync, fmt.Errorf("unmarshal procError failed: %w", err)
		}
		return sync, &ierr
	}
	if sync.Flags&syncFlagHasFd != 0 {
		file, err := utils.RecvFile(pipe)
		if err != nil {
			return sync, fmt.Errorf("receiving fd from sync %q failed: %w", sync.Type, err)
		}
		sync.File = file
	}
	return sync, nil
}

func readSyncFull(pipe *os.File, expected syncType) (syncT, error) {
	sync, err := doReadSync(pipe)
	if err != nil {
		return sync, err
	}
	if sync.Type != expected {
		return sync, fmt.Errorf("unexpected synchronisation flag: got %q, expected %q", sync.Type, expected)
	}
	return sync, nil
}

func readSync(pipe *os.File, expected syncType) error {
	sync, err := readSyncFull(pipe, expected)
	if err != nil {
		return err
	}
	if sync.Arg != nil {
		return fmt.Errorf("sync %q had unexpected argument passed: %q", expected, string(*sync.Arg))
	}
	if sync.File != nil {
		_ = sync.File.Close()
		return fmt.Errorf("sync %q had unexpected file passed", sync.Type)
	}
	return nil
}

// parseSync runs the given callback function on each syncT received from the
// child. It will return once io.EOF is returned from the given pipe.
func parseSync(pipe *os.File, fn func(*syncT) error) error {
	for {
		sync, err := doReadSync(pipe)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := fn(&sync); err != nil {
			return err
		}
	}
	return nil
}

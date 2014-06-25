package libcontainer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State represents a running container's state
type State struct {
	// InitPid is the init process id in the parent namespace
	InitPid int `json:"init_pid,omitempty"`
	// InitStartTime is the init process start time
	InitStartTime string `json:"init_start_time,omitempty"`
}

// SaveState writes the container's runtime state to a state.json file
// in the specified path
func SaveState(basePath string, state *State) error {
	f, err := os.Create(filepath.Join(basePath, "state.json"))
	if err != nil {
		return err
	}
	defer f.Close()

	return json.NewEncoder(f).Encode(state)
}

// GetState reads the state.json file for a running container
func GetState(basePath string) (*State, error) {
	f, err := os.Open(filepath.Join(basePath, "state.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var state *State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, err
	}

	return state, nil
}

// DeleteState deletes the state.json file
func DeleteState(basePath string) error {
	return os.Remove(filepath.Join(basePath, "state.json"))
}

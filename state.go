package libcontainer

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State represents a running container's state
type State struct {
	// Pid1 is the process id for the container's pid 1 in it's parent namespace
	Pid1 int `json:"pid1,omitempty"`
	// Pid1StartTime is the process start time for the container's pid 1
	Pid1StartTime string `json:"pid1_start_time,omitempty"`
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

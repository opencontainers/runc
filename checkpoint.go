package libcontainer

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/docker/libcontainer/network"
)

// A checkpoint struct that will contain all runtime checkpointed information.
type RuntimeCkpt struct {
	NetworkCkpt network.NetworkCkpt `json:"network_ckpt,omitempty"`
}

// The name of the network checkpoint file
const runtimeCkptFile = "runtimeCkpt.json"

var ErrRuntimeCkptNotFound = errors.New("Runtime Checkpoint not found")

// Returns the path to the network checkpoint given the path to the base directory of network checkpoint.
func getRuntimeCkptPath(basePath string) string {
	return filepath.Join(basePath, runtimeCkptFile)
}

// Updates the Runtime Checkpoint with current checkpoint information from all the subsystems.
func UpdateRuntimeCkpt(basePath string) error {
	runtimeCkpt := &RuntimeCkpt{
		NetworkCkpt: *network.NetworkCkptImpl.GetNetworkCkpt(),
	}
	data, err := json.Marshal(runtimeCkpt)
	if err != nil {
		return fmt.Errorf("Failed to checkpoint runtime information - %s", err)
	}
	return ioutil.WriteFile(getRuntimeCkptPath(basePath), data, 0655)
}

// Loads and returns the rutime checkpointing existing inside basePath. Returns ErrRuntimeCkptNotFound
// if the runtime checkpoint does not exist.
func GetRuntimeCkpt(basePath string) (*RuntimeCkpt, error) {
	runtimeCkpt := &RuntimeCkpt{}
	checkpointPath := getRuntimeCkptPath(basePath)
	f, err := os.Open(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return runtimeCkpt, ErrRuntimeCkptNotFound
		}
		return runtimeCkpt, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(runtimeCkpt); err != nil {
		return runtimeCkpt, err
	}

	return runtimeCkpt, nil
}

// Deletes the runtime checkpoint under basePath
func DeleteNetworkRuntimeInfo(basePath string) error {
	return os.Remove(getRuntimeCkptPath(basePath))
}

package network

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Struct describing the network specific checkpoint that will be maintained by libcontainer for all running containers
// This is an internal checkpoint, so do not depend on it outside of libcontainer.
type NetworkRuntimeInfo struct {
	// The name of the veth interface on the Host.
	VethHost string `json:"veth_host,omitempty"`
	// The name of the veth interface created inside the container for the child.
	VethChild string `json:"veth_child,omitempty"`
}

// The name of the network checkpoint file
const networkInfoFile = "network.json"

var ErrNetworkRuntimeInfoNotFound = errors.New("Network Checkpoint not found")

// Returns the path to the network checkpoint given the path to the base directory of network checkpoint.
func getNetworkRuntimeInfoPath(basePath string) string {
	return filepath.Join(basePath, networkInfoFile)
}

// Marshalls the input network runtime info struct into a json object and stores it inside basepath.
func writeNetworkRuntimeInfo(networkInfo *NetworkRuntimeInfo, basePath string) error {
	data, err := json.Marshal(networkInfo)
	if err != nil {
		return fmt.Errorf("Failed to checkpoint network runtime information - %s", err)
	}
	return ioutil.WriteFile(getNetworkRuntimeInfoPath(basePath), data, 0655)
}

// Loads the network runtime info from the checkpoint and returns the unmarshaled content.
func LoadNetworkRuntimeInfo(basePath string) (NetworkRuntimeInfo, error) {
	var networkRuntimeInfo NetworkRuntimeInfo
	checkpointPath := getNetworkRuntimeInfoPath(basePath)
	f, err := os.Open(checkpointPath)
	if err != nil {
		if os.IsNotExist(err) {
			return networkRuntimeInfo, ErrNetworkRuntimeInfoNotFound
		}
		return networkRuntimeInfo, err
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(&networkRuntimeInfo); err != nil {
		return networkRuntimeInfo, err
	}

	return networkRuntimeInfo, nil
}

// Deletes the network checkpoint under basePath
func deleteNetworkRuntimeInfo(basePath string) error {
	return os.Remove(getNetworkRuntimeInfoPath(basePath))
}

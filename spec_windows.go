package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

type WindowsSpec struct {
	PortableSpec
}

// loadSpec loads the specification from the provided path.
// If the path is empty then the default path will be "container.json"
func loadSpec(path string) (*WindowsSpec, error) {
	if path == "" {
		path = "container.json"
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file for %s not found", path)
		}
		return nil, err
	}
	defer f.Close()
	var s *WindowsSpec
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}
	return s, nil
}

func createLibcontainerConfig(spec *WindowsSpec) (*configs.Config, error) {
	config := &configs.Config{}
	return config, nil
}

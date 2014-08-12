package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/syncpipe"
)

// rFunc is a function registration for calling after an execin
type rFunc struct {
	Usage  string
	Action func(*libcontainer.Config, []string)
}

func loadContainer() (*libcontainer.Config, error) {
	f, err := os.Open(filepath.Join(dataPath, "container.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var container *libcontainer.Config
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}

	return container, nil
}

func openLog(name string) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0755)
	if err != nil {
		return err
	}

	log.SetOutput(f)

	return nil
}

func loadContainerFromJson(rawData string) (*libcontainer.Config, error) {
	var container *libcontainer.Config

	if err := json.Unmarshal([]byte(rawData), &container); err != nil {
		return nil, err
	}

	return container, nil
}

func findUserArgs() []string {
	i := 0
	for _, a := range os.Args {
		i++

		if a == "--" {
			break
		}
	}

	return os.Args[i:]
}

// loadConfigFromFd loads a container's config from the sync pipe that is provided by
// fd 3 when running a process
func loadConfigFromFd() (*libcontainer.Config, error) {
	syncPipe, err := syncpipe.NewSyncPipeFromFd(0, 3)
	if err != nil {
		return nil, err
	}

	var config *libcontainer.Config
	if err := syncPipe.ReadFromParent(&config); err != nil {
		return nil, err
	}

	return config, nil
}

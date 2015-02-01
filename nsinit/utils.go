package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/codegangsta/cli"
	"github.com/docker/libcontainer"
	"github.com/docker/libcontainer/configs"
)

func loadConfig() (*configs.Config, error) {
	f, err := os.Open(filepath.Join(dataPath, "container.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var container *configs.Config
	if err := json.NewDecoder(f).Decode(&container); err != nil {
		return nil, err
	}
	return container, nil
}

// loadConfigFromFd loads a container's config from the sync pipe that is provided by
// fd 3 when running a process
func loadConfigFromFd() (*configs.Config, error) {
	pipe := os.NewFile(3, "pipe")
	defer pipe.Close()

	var config *configs.Config
	if err := json.NewDecoder(pipe).Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}

func loadFactory(context *cli.Context) (libcontainer.Factory, error) {
	return libcontainer.New(context.GlobalString("root"), []string{os.Args[0], "init", "--fd", "3", "--"})
}

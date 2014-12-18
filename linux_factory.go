// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"
)

const (
	configFilename = "config.json"
	stateFilename  = "state.json"
)

var (
	idRegex  = regexp.MustCompile(`^[\w_]+$`)
	maxIdLen = 1024
)

// New returns a linux based container factory based in the root directory.
func New(root string, initArgs []string) (Factory, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, newGenericError(err, SystemError)
		}
	}

	return &linuxFactory{
		root:     root,
		initArgs: initArgs,
	}, nil
}

// linuxFactory implements the default factory interface for linux based systems.
type linuxFactory struct {
	// root is the root directory
	root     string
	initArgs []string
}

func (l *linuxFactory) Create(id string, config *Config) (Container, error) {
	if l.root == "" {
		return nil, newGenericError(fmt.Errorf("invalid root"), ConfigInvalid)
	}
	if !idRegex.MatchString(id) {
		return nil, newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}

	if len(id) > maxIdLen {
		return nil, newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}

	containerRoot := filepath.Join(l.root, id)
	_, err := os.Stat(containerRoot)
	if err == nil {
		return nil, newGenericError(fmt.Errorf("Container with id exists: %v", id), IdInUse)
	} else if !os.IsNotExist(err) {
		return nil, newGenericError(err, SystemError)
	}

	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}

	if err := os.MkdirAll(containerRoot, 0700); err != nil {
		return nil, newGenericError(err, SystemError)
	}

	f, err := os.Create(filepath.Join(containerRoot, configFilename))
	if err != nil {
		os.RemoveAll(containerRoot)
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		os.RemoveAll(containerRoot)
		return nil, newGenericError(err, SystemError)
	}

	cgroupManager := NewCgroupManager()
	return &linuxContainer{
		id:            id,
		root:          containerRoot,
		config:        config,
		initArgs:      l.initArgs,
		state:         &State{},
		cgroupManager: cgroupManager,
	}, nil
}

func (l *linuxFactory) Load(id string) (Container, error) {
	if l.root == "" {
		return nil, newGenericError(fmt.Errorf("invalid root"), ConfigInvalid)
	}
	containerRoot := filepath.Join(l.root, id)
	glog.Infof("loading container config from %s", containerRoot)
	config, err := l.loadContainerConfig(containerRoot)
	if err != nil {
		return nil, err
	}

	glog.Infof("loading container state from %s", containerRoot)
	state, err := l.loadContainerState(containerRoot)
	if err != nil {
		return nil, err
	}

	cgroupManager := NewCgroupManager()
	glog.Infof("using %s as cgroup manager", cgroupManager)
	return &linuxContainer{
		id:            id,
		root:          containerRoot,
		config:        config,
		state:         state,
		cgroupManager: cgroupManager,
		initArgs:      l.initArgs,
	}, nil
}

func (l *linuxFactory) loadContainerConfig(root string) (*Config, error) {
	f, err := os.Open(filepath.Join(root, configFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newGenericError(err, ContainerNotExists)
		}
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()

	var config *Config
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, newGenericError(err, ConfigInvalid)
	}
	return config, nil
}

func (l *linuxFactory) loadContainerState(root string) (*State, error) {
	f, err := os.Open(filepath.Join(root, stateFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newGenericError(err, ContainerNotExists)
		}
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()

	var state *State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return state, nil
}

// StartInitialization loads a container by opening the pipe fd from the parent to read the configuration and state
// This is a low level implementation detail of the reexec and should not be consumed externally
func (f *linuxFactory) StartInitialization(pipefd uintptr) (err error) {

	/* FIXME call namespaces.Init() */
	return nil
}

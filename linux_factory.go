// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Sirupsen/logrus"
)

const (
	configFilename = "config.json"
	stateFilename  = "state.json"
)

var (
	idRegex = regexp.MustCompile(`^[\w_]{1,1024}$`)
)

// New returns a linux based container factory based in the root directory.
func New(root string, logger *logrus.Logger) (Factory, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, newGenericError(err, SystemError)
	}

	return &linuxFactory{
		root:   root,
		logger: logger,
	}, nil
}

// linuxFactory implements the default factory interface for linux based systems.
type linuxFactory struct {
	// root is the root directory
	root string

	// standard logger for all packages
	logger *logrus.Logger
}

func (l *linuxFactory) Create(id string, config *Config) (Container, error) {
	if !idRegex.MatchString(id) {
		return nil, newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}

	containerRoot := filepath.Join(l.root, id)
	if _, err := os.Stat(containerRoot); err == nil {
		return nil, newGenericError(fmt.Errorf("Container with id exists: %v", id), IdInUse)
	}

	panic("not implemented")
}

func (l *linuxFactory) Load(id string) (Container, error) {
	containerRoot := filepath.Join(l.root, id)
	l.logger.Debugf("loading container config from %s", containerRoot)
	config, err := l.loadContainerConfig(containerRoot)
	if err != nil {
		return nil, err
	}

	l.logger.Debugf("loading container state from %s", containerRoot)
	state, err := l.loadContainerState(containerRoot)
	if err != nil {
		return nil, err
	}

	cgroupManager := newCgroupsManager()
	l.logger.Debugf("using %s as cgroup manager", cgroupManager)
	return &linuxContainer{
		id:            id,
		root:          containerRoot,
		config:        config,
		state:         state,
		cgroupManager: cgroupManager,
		logger:        l.logger,
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

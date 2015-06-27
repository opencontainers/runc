package libcontainer

import (
	"fmt"
	"os"
	"regexp"

	"github.com/opencontainers/runc/libcontainer/configs"
)

const (
	stateFilename = "state.json"
)

var (
	idRegex  = regexp.MustCompile(`^[\w_-]+$`)
	maxIdLen = 1024
)

// InitArgs returns an options func to configure a WindowsFactory with the
// provided init arguments.
func InitArgs(args ...string) func(*WindowsFactory) error {
	return func(l *WindowsFactory) error {
		return nil
	}
}

// New returns a windows based container factory based in the root directory and
// configures the factory with the provided option funcs.
func New(root string, options ...func(*WindowsFactory) error) (Factory, error) {
	l := &WindowsFactory{}
	InitArgs(os.Args[0], "init")(l)
	for _, opt := range options {
		if err := opt(l); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// WindowsFactory implements the default factory interface for Windows-based systems.
type WindowsFactory struct {
}

func (l *WindowsFactory) Create(id string, config *configs.Config) (Container, error) {
	return &windowsContainer{
		id:     id,
		config: config,
	}, nil
}

func (l *WindowsFactory) Load(id string) (Container, error) {
	return &windowsContainer{
		id: id,
	}, nil
}

func (l *WindowsFactory) Type() string {
	return "libcontainer"
}

// StartInitialization loads a container by opening the pipe fd from the parent to read the configuration and state
// This is a low level implementation detail of the reexec and should not be consumed externally
func (l *WindowsFactory) StartInitialization() (err error) {
	return nil
}

func (l *WindowsFactory) validateID(id string) error {
	if !idRegex.MatchString(id) {
		return newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}
	if len(id) > maxIdLen {
		return newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}
	return nil
}

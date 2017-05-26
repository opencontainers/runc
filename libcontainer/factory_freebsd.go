package libcontainer

import (
	"fmt"
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func New(root string, options ...func(*FreeBSDFactory) error) (Factory, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, newGenericError(err, SystemError)
		}
	}

	l := &FreeBSDFactory{
		Root: root,
	}

	return l, nil
}

type FreeBSDFactory struct {
	// Root directory for the factory to store state.
	Root string
}

func (l *FreeBSDFactory) Create(id string, config *configs.Config) (Container, error) {
	if l.Root == "" {
		return nil, newGenericError(fmt.Errorf("invalid root"), ConfigInvalid)
	}

	c := &freebsdContainer{
		id: id,
	}
	return c, nil
}

func (l *FreeBSDFactory) Load(id string) (Container, error) {
	return nil, nil
}

func (l *FreeBSDFactory) Type() string {
	return "libcontainer"
}

func (l *FreeBSDFactory) StartInitialization() (err error) {
	return nil
}

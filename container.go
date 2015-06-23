package runc

import (
	"path/filepath"

	"github.com/docker/libcontainer"
)

// NewFactory returns the configured libcontainer.Factory instance for execing containers.
func NewFactory(root string, criu string) (libcontainer.Factory, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return libcontainer.New(abs, libcontainer.Cgroupfs, func(l *libcontainer.LinuxFactory) error {
		l.CriuPath = criu
		return nil
	})
}

// GetContainer returns the specified container instance by loading it from state
// with the default factory.
func GetContainer(factory libcontainer.Factory, id string) (libcontainer.Container, error) {
	container, err := factory.Load(id)
	if err != nil {
		return nil, err
	}
	return container, nil
}

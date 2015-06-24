package runc

import (
	"path/filepath"

	"github.com/opencontainers/runc/libcontainer"
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

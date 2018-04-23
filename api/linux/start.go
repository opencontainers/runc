package linux

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runc/libcontainer"
)

func (l *Libcontainer) Start(id string) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	status, err := container.Status()
	if err != nil {
		return err
	}
	switch status {
	case libcontainer.Created:
		return container.Exec()
	case libcontainer.Stopped:
		return errors.New("cannot start a container that has stopped")
	case libcontainer.Running:
		return errors.New("cannot start an already running container")
	default:
		return fmt.Errorf("cannot start a container in the %s state\n", status)
	}
}

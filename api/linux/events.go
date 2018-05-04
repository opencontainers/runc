package linux

import (
	"context"

	"github.com/opencontainers/runc/libcontainer"
)

func (l *Libcontainer) NotifyOOM(ctx context.Context, id string) (<-chan struct{}, error) {
	container, err := l.getContainer(id)
	if err != nil {
		return nil, err
	}
	return container.NotifyOOM()
}

func (l *Libcontainer) Stats(ctx context.Context, id string) (*libcontainer.Stats, error) {
	container, err := l.getContainer(id)
	if err != nil {
		return nil, err
	}
	return container.Stats()
}

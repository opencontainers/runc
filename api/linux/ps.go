package linux

import (
	"context"
	"fmt"
)

func (l *Libcontainer) PS(ctx context.Context, id string) ([]int, error) {
	// XXX: Currently not supported with rootless containers.
	if isRootless() {
		return nil, fmt.Errorf("runc ps requires root")
	}
	container, err := l.getContainer(id)
	if err != nil {
		return nil, err
	}
	return container.Processes()
}

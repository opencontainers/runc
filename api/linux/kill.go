package linux

import (
	"context"
	"os"

	"github.com/opencontainers/runc/api"
)

func (l *Libcontainer) Kill(ctx context.Context, id string, signal os.Signal, opts api.KillOpts) error {
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	return container.Signal(signal, opts.All)
}

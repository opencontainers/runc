package linux

import (
	"context"

	"github.com/opencontainers/runc/api"
)

func (l *Libcontainer) Run(ctx context.Context, id string, opts api.CreateOpts) (*api.CreateResult, error) {
	status, err := l.startContainer(id, opts, CT_ACT_RUN, nil)
	if err != nil {
		return nil, err
	}
	return &api.CreateResult{
		Status: status,
	}, nil
}

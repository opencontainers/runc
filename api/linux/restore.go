package linux

import (
	"context"
	"fmt"

	"github.com/opencontainers/runc/api"
)

func (l *Libcontainer) Restore(ctx context.Context, id string, opts api.RestoreOpts) (*api.CreateResult, error) {
	// XXX: Currently this is untested with rootless containers.
	if isRootless() {
		return nil, fmt.Errorf("runc restore requires root")
	}
	options, err := criuOptions(opts.CheckpointOpts)
	if err != nil {
		return nil, err
	}
	// clear managed cgroups mode on restore
	options.ManageCgroupsMode = 0
	status, err := l.startContainer(id, opts.CreateOpts, CT_ACT_RESTORE, options)
	if err != nil {
		return nil, err
	}
	return &api.CreateResult{
		Status: status,
	}, nil
}

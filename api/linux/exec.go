package linux

import (
	"context"
	"fmt"

	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/libcontainer"
)

func (l *Libcontainer) Exec(ctx context.Context, id string, opts api.ExecOpts) (*api.CommandResult, error) {
	container, err := l.getContainer(id)
	if err != nil {
		return nil, err
	}
	status, err := container.Status()
	if err != nil {
		return nil, err
	}
	if status == libcontainer.Stopped {
		return nil, fmt.Errorf("cannot exec a container that has stopped")
	}
	r := &runner{
		enableSubreaper: false,
		shouldDestroy:   false,
		container:       container,
		consoleSocket:   opts.ConsoleSocket,
		detach:          opts.Detach,
		pidFile:         opts.PidFile,
		action:          CT_ACT_RUN,
	}
	s, err := r.run(opts.Process, opts.Stdin, opts.Stdout, opts.Stderr)
	if err != nil {
		return nil, err
	}
	return &api.CommandResult{
		Status: s,
	}, nil
}

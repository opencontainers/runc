package linux

import (
	"fmt"

	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/libcontainer"
)

func (l *Libcontainer) Checkpoint(id string, opts api.CheckpointOpts) error {
	// XXX: Currently this is untested with rootless containers.
	if isRootless() {
		return fmt.Errorf("checkpoint requires root")
	}
	container, err := l.getContainer(id)
	if err != nil {
		return err
	}
	status, err := container.Status()
	if err != nil {
		return err
	}
	if status == libcontainer.Created || status == libcontainer.Stopped {
		return fmt.Errorf("Container cannot be checkpointed in %s state", status.String())
	}
	defer destroy(container)

	options, err := criuOptions(opts)
	if err != nil {
		return err
	}
	return container.Checkpoint(options)
}

func criuOptions(opts api.CheckpointOpts) (*libcontainer.CriuOpts, error) {
	mode := libcontainer.CRIU_CG_MODE_DEFAULT
	if opts.ManageCgroupsMode != "" {
		switch opts.ManageCgroupsMode {
		case "soft":
			mode = libcontainer.CRIU_CG_MODE_SOFT
		case "full":
			mode = libcontainer.CRIU_CG_MODE_FULL
		case "strict":
			mode = libcontainer.CRIU_CG_MODE_STRICT
		default:
			return nil, fmt.Errorf("invalid manage cgroups mode %s", opts.ManageCgroupsMode)
		}
	}
	return &libcontainer.CriuOpts{
		ImagesDirectory:         opts.ImagesDirectory,
		WorkDirectory:           opts.WorkDirectory,
		ParentImage:             opts.ParentImage,
		LeaveRunning:            opts.LeaveRunning,
		TcpEstablished:          opts.TcpEstablished,
		ExternalUnixConnections: opts.ExternalUnixConnections,
		ShellJob:                opts.ShellJob,
		FileLocks:               opts.FileLocks,
		PreDump:                 opts.PreDump,
		AutoDedup:               opts.AutoDedup,
		LazyPages:               opts.LazyPages,
		StatusFd:                opts.StatusFd,
		ManageCgroupsMode:       mode,
	}, nil
}

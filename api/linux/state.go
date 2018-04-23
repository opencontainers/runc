package linux

import (
	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
)

func (l *Libcontainer) State(id string) (*api.Container, error) {
	container, err := l.getContainer(id)
	if err != nil {
		return nil, err
	}
	containerStatus, err := container.Status()
	if err != nil {
		return nil, err
	}
	state, err := container.State()
	if err != nil {
		return nil, err
	}
	pid := state.BaseState.InitProcessPid
	if containerStatus == libcontainer.Stopped {
		pid = 0
	}
	bundle, annotations := utils.Annotations(state.Config.Labels)
	return &api.Container{
		Version:        state.BaseState.Config.Version,
		ID:             state.BaseState.ID,
		InitProcessPid: pid,
		Status:         containerStatus.String(),
		Bundle:         bundle,
		Rootfs:         state.BaseState.Config.Rootfs,
		Created:        state.BaseState.Created,
		Annotations:    annotations,
	}, nil
}

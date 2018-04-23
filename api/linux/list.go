package linux

import (
	"fmt"
	"io/ioutil"
	"os"
	"syscall"

	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/user"
	"github.com/opencontainers/runc/libcontainer/utils"
)

func (l *Libcontainer) List() ([]api.Container, error) {
	factory, err := l.loadFactory()
	if err != nil {
		return nil, err
	}
	list, err := ioutil.ReadDir(l.root)
	if err != nil {
		return nil, err
	}
	var s []api.Container
	for _, item := range list {
		if item.IsDir() {
			// This cast is safe on Linux.
			stat := item.Sys().(*syscall.Stat_t)
			owner, err := user.LookupUid(int(stat.Uid))
			if err != nil {
				owner.Name = fmt.Sprintf("#%d", stat.Uid)
			}
			container, err := factory.Load(item.Name())
			if err != nil {
				fmt.Fprintf(os.Stderr, "load container %s: %v\n", item.Name(), err)
				continue
			}
			containerStatus, err := container.Status()
			if err != nil {
				fmt.Fprintf(os.Stderr, "status for %s: %v\n", item.Name(), err)
				continue
			}
			state, err := container.State()
			if err != nil {
				fmt.Fprintf(os.Stderr, "state for %s: %v\n", item.Name(), err)
				continue
			}
			pid := state.BaseState.InitProcessPid
			if containerStatus == libcontainer.Stopped {
				pid = 0
			}
			bundle, annotations := utils.Annotations(state.Config.Labels)
			s = append(s, api.Container{
				Version:        state.BaseState.Config.Version,
				ID:             state.BaseState.ID,
				InitProcessPid: pid,
				Status:         containerStatus.String(),
				Bundle:         bundle,
				Rootfs:         state.BaseState.Config.Rootfs,
				Created:        state.BaseState.Created,
				Annotations:    annotations,
				Owner:          owner.Name,
			})
		}
	}
	return s, nil
}

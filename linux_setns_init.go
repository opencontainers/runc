// +build linux

package libcontainer

import (
	"github.com/docker/libcontainer/apparmor"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/system"
)

// linuxSetnsInit performs the container's initialization for running a new process
// inside an existing container.
type linuxSetnsInit struct {
	args   []string
	env    []string
	config *configs.Config
}

func (l *linuxSetnsInit) Init() error {
	if err := setupRlimits(l.config); err != nil {
		return err
	}
	if err := finalizeNamespace(l.config); err != nil {
		return err
	}
	if err := apparmor.ApplyProfile(l.config.AppArmorProfile); err != nil {
		return err
	}
	if l.config.ProcessLabel != "" {
		if err := label.SetProcessLabel(l.config.ProcessLabel); err != nil {
			return err
		}
	}
	return system.Execv(l.args[0], l.args[0:], l.env)
}

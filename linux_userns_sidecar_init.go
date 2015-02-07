// +build linux

package libcontainer

import (
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/mount"
)

// linuxUsernsSideCar is run to setup mounts and networking related operations
// for a user namespace enabled process as a user namespace root doesn't
// have permissions to perform these operations.
// The setup process joins all the namespaces of user namespace enabled init
// except the user namespace, so it run as root in the root user namespace
// to perform these operations.
type linuxUsernsSideCar struct {
	config *configs.Config
}

func (l *linuxUsernsSideCar) Init() error {
	if err := setupNetwork(l.config); err != nil {
		return err
	}
	if err := setupRoute(l.config); err != nil {
		return err
	}
	label.Init()
	// InitializeMountNamespace() can be executed only for a new mount namespace
	if l.config.Namespaces.Contains(configs.NEWNET) {
		if err := mount.InitializeMountNamespace(l.config); err != nil {
			return err
		}
	}
	return nil
}

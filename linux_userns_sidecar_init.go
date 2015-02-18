// +build linux

package libcontainer

// linuxUsernsSideCar is run to setup mounts and networking related operations
// for a user namespace enabled process as a user namespace root doesn't
// have permissions to perform these operations.
// The setup process joins all the namespaces of user namespace enabled init
// except the user namespace, so it run as root in the root user namespace
// to perform these operations.
type linuxUsernsSideCar struct {
	config *initConfig
}

func (l *linuxUsernsSideCar) Init() error {
	if err := setupNetwork(l.config); err != nil {
		return err
	}
	if err := setupRoute(l.config.Config); err != nil {
		return err
	}
	return nil
}

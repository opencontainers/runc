package libcontainer

import (
	"os"

	"github.com/opencontainers/runc/libcontainer/configs"
)

type freebsdContainer struct {
	id     string
	root   string
	config *configs.Config
}

// State represents a running container's state
type State struct {
	BaseState

	// Platform specific fields below here
}

// A libcontainer container object.
//
// Each container is thread-safe within the same process. Since a container can
// be destroyed by a separate process, any function may return that the container
// was not found.
type Container interface {
	BaseContainer

	// Methods below here are platform specific

}

// Config returns the container's configuration
func (c *freebsdContainer) Config() configs.Config {
	return *c.config
}

func (c *freebsdContainer) Destroy() error {
	return nil
}

func (c *freebsdContainer) Exec() error {
	return nil
}

func (c *freebsdContainer) ID() string {
	return c.id
}

func (c *freebsdContainer) Processes() ([]int, error) {
	return nil, nil
}

func (c *freebsdContainer) Run(process *Process) error {
	return nil
}

func (c *freebsdContainer) Set(config configs.Config) error {
	return nil
}

func (c *freebsdContainer) Signal(s os.Signal) error {
	return nil
}

func (c *freebsdContainer) Start(process *Process) error {
	return nil
}

func (c *freebsdContainer) State() (*State, error) {
	return nil, nil
}

func (c *freebsdContainer) Stats() (*Stats, error) {
	return nil, nil
}

func (c *freebsdContainer) Status() (Status, error) {
	return 0, nil
}

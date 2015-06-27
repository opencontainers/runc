// +build windows

package libcontainer

import (
	"sync"

	"github.com/opencontainers/runc/libcontainer/configs"
)

type windowsContainer struct {
	id       string
	root     string
	config   *configs.Config
	initPath string
	initArgs []string
	m        sync.Mutex
}

// ID returns the container's unique ID
func (c *windowsContainer) ID() string {
	return c.id
}

// Config returns the container's configuration
func (c *windowsContainer) Config() configs.Config {
	return *c.config
}

func (c *windowsContainer) Status() (Status, error) {
	return 0, nil
}

func (c *windowsContainer) State() (*State, error) {
	return nil, nil
}

func (c *windowsContainer) Processes() ([]int, error) {
	return nil, nil
}

func (c *windowsContainer) Stats() (*Stats, error) {
	return nil, nil
}

func (c *windowsContainer) Set(config configs.Config) error {
	return nil
}

func (c *windowsContainer) Start(process *Process) error {
	return nil
}

func (c *windowsContainer) newInitConfig(process *Process) *initConfig {
	return &initConfig{
		Config: c.config,
		Args:   process.Args,
		Env:    process.Env,
		User:   process.User,
		Cwd:    process.Cwd,
	}
}

func (c *windowsContainer) Destroy() error {
	return nil
}

func (c *windowsContainer) Pause() error {
	return nil
}

func (c *windowsContainer) Resume() error {
	return nil
}

// TODO Windows. This needs further refactoring. Linux is not platform specific in
// this method as criuOpts is Linux specific.
func (c *windowsContainer) Checkpoint(criuOpts *CriuOpts) error {
	return nil
}

func (c *windowsContainer) NotifyOOM() (<-chan struct{}, error) {
	return nil, nil
}

// TODO Windows. This needs further refactoring. Linux is not platform specific in
// this method as criuOpts is Linux specific.
func (c *windowsContainer) Restore(process *Process, criuOpts *CriuOpts) error {
	return nil
}

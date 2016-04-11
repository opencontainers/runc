// +build nocriu

package libcontainer

import (
	"errors"
)

var errNotEnabled = errors.New("Checkpoint/restore not supported")

func (c *linuxContainer) Checkpoint(opts *CheckpointOpts) error {
	return errNotEnabled
}

func (c *linuxContainer) Restore(process *Process, opts *CheckpointOpts) error {
	return errNotEnabled
}

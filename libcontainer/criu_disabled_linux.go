//go:build runc_nocriu

package libcontainer

import "errors"

var ErrNoCR = errors.New("this runc binary has not been compiled with checkpoint/restore support enabled (runc_nocriu)")

func (c *Container) Restore(process *Process, criuOpts *CriuOpts) error {
	return ErrNoCR
}

func (c *Container) Checkpoint(criuOpts *CriuOpts) error {
	return ErrNoCR
}

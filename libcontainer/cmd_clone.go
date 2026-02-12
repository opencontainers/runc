package libcontainer

import "os/exec"

// cloneCmd creates a copy of exec.Cmd. It is needed because cmd.Start
// must only be used once, and go1.26 actually enforces that (see
// https://go-review.googlesource.com/c/go/+/728642). The implementation
// is similar to
//
//	cmd = *c
//	return &cmd
//
// except it does not copy private fields, or fields populated
// after the call to cmd.Start.
//
// NOTE if Go will add exec.Cmd.Clone, we should switch to it.
func cloneCmd(c *exec.Cmd) *exec.Cmd {
	cmd := &exec.Cmd{
		Path:        c.Path,
		Args:        c.Args,
		Env:         c.Env,
		Dir:         c.Dir,
		Stdin:       c.Stdin,
		Stdout:      c.Stdout,
		Stderr:      c.Stderr,
		ExtraFiles:  c.ExtraFiles,
		SysProcAttr: c.SysProcAttr,
		// Don't copy Process, ProcessState, Err since
		// these fields are populated after the start.

		// Technically, we do not use Cancel or WaitDelay,
		// but they are here for the sake of completeness.
		Cancel:    c.Cancel,
		WaitDelay: c.WaitDelay,
	}
	return cmd
}

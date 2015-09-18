// +build linux

package main

import (
	"fmt"
	"io"
	"os"
	"syscall"

	"github.com/docker/docker/pkg/term"
	"github.com/opencontainers/runc/libcontainer"
)

// newTty creates a new tty for use with the container.  If a tty is not to be
// created for the process, pipes are created so that the TTY of the parent
// process are not inherited by the container.
func newTty(create bool, p *libcontainer.Process, rootuid int) (*tty, error) {
	if create {
		return createTty(p, rootuid)
	}
	return createStdioPipes(p, rootuid)
}

// setup standard pipes so that the TTY of the calling runc process
// is not inherited by the container.
func createStdioPipes(p *libcontainer.Process, rootuid int) (*tty, error) {
	var (
		t   = &tty{}
		fds []int
	)
	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	go io.Copy(w, os.Stdin)
	t.closers = append(t.closers, w)
	p.Stdin = r
	if r, w, err = os.Pipe(); err != nil {
		return nil, err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	go io.Copy(os.Stdout, r)
	p.Stdout = w
	t.closers = append(t.closers, r)
	if r, w, err = os.Pipe(); err != nil {
		return nil, err
	}
	fds = append(fds, int(r.Fd()), int(w.Fd()))
	go io.Copy(os.Stderr, r)
	p.Stderr = w
	t.closers = append(t.closers, r)
	// change the ownership of the pipe fds incase we are in a user namespace.
	for _, fd := range fds {
		if err := syscall.Fchown(fd, rootuid, rootuid); err != nil {
			return nil, err
		}
	}
	return t, nil
}

func createTty(p *libcontainer.Process, rootuid int) (*tty, error) {
	console, err := p.NewConsole(rootuid)
	if err != nil {
		return nil, err
	}
	go io.Copy(console, os.Stdin)
	go io.Copy(os.Stdout, console)
	state, err := term.SetRawTerminal(os.Stdin.Fd())
	if err != nil {
		return nil, fmt.Errorf("failed to set the terminal from the stdin: %v", err)
	}
	t := &tty{
		console: console,
		state:   state,
		closers: []io.Closer{
			console,
		},
	}
	p.Stderr = nil
	p.Stdout = nil
	p.Stdin = nil
	return t, nil
}

type tty struct {
	console libcontainer.Console
	state   *term.State
	closers []io.Closer
}

func (t *tty) Close() error {
	for _, c := range t.closers {
		c.Close()
	}
	if t.state != nil {
		term.RestoreTerminal(os.Stdin.Fd(), t.state)
	}
	return nil
}

func (t *tty) resize() error {
	if t.console == nil {
		return nil
	}
	ws, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return err
	}
	return term.SetWinsize(t.console.Fd(), ws)
}

package main

import (
	"io"
	"os"

	"github.com/codegangsta/cli"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/libcontainer"
)

func newTty(context *cli.Context) (*tty, error) {
	if context.Bool("tty") {
		console, err := libcontainer.NewConsole()
		if err != nil {
			return nil, err
		}
		go io.Copy(console, os.Stdin)
		go io.Copy(os.Stdout, console)
		state, err := term.SetRawTerminal(os.Stdin.Fd())
		if err != nil {
			return nil, err
		}
		return &tty{
			console: console,
			state:   state,
		}, nil
	}
	return &tty{}, nil
}

type tty struct {
	console libcontainer.Console
	state   *term.State
}

func (t *tty) Close() error {
	if t.console != nil {
		t.console.Close()
	}
	if t.state != nil {
		term.RestoreTerminal(os.Stdin.Fd(), t.state)
	}
	return nil
}

func (t *tty) attach(process *libcontainer.Process) {
	if t.console != nil {
		process.Stderr = nil
		process.Stdout = nil
		process.Stdin = nil
	}
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

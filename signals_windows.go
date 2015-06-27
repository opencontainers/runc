package main

import (
	"os"
	"os/signal"

	"github.com/opencontainers/runc/libcontainer"
)

// TODO The implementation on Windows will look different. Windows only handles
// the SIGTERM signal.

const signalBufferSize = 2048

// newSignalHandler returns a signal handler for processing SIGCHLD and SIGWINCH signals
// while still forwarding all other signals to the process.
func newSignalHandler(tty *tty) *signalHandler {
	// ensure that we have a large buffer size so that we do not miss any signals
	// incase we are not processing them fast enough.
	s := make(chan os.Signal, signalBufferSize)
	// handle all signals for the process.
	signal.Notify(s)
	return &signalHandler{
		tty:     tty,
		signals: s,
	}
}

// exit models a process exit status with the pid and
// exit status.
type exit struct {
	pid    int
	status int
}

type signalHandler struct {
	signals chan os.Signal
	tty     *tty
}

// forward handles the main signal event loop forwarding, resizing, or reaping depeding
// on the signal received.
func (h *signalHandler) forward(process *libcontainer.Process) (int, error) {
	return 0, nil
}

// reap runs wait4 in a loop until we have finished processing any existing exits
// then returns all exits to the main event loop for further processing.
func (h *signalHandler) reap() (exits []exit, err error) {
	return nil, nil
}

func (h *signalHandler) Close() error {
	return nil
}

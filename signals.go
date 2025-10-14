package main

import (
	"os"
	"os/signal"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const signalBufferSize = 2048

// newSignalHandler returns a signal handler for processing SIGCHLD and SIGWINCH signals
// while still forwarding all other signals to the process.
func newSignalHandler() chan *signalHandler {
	handler := make(chan *signalHandler)
	// signal.Notify is actually quite expensive, as it has to configure the
	// signal mask and add signal handlers for all signals (all ~65 of them).
	// So, defer this to a background thread while doing the rest of the io/tty
	// setup.
	go func() {
		// ensure that we have a large buffer size so that we do not miss any
		// signals in case we are not processing them fast enough.
		s := make(chan os.Signal, signalBufferSize)
		// handle all signals for the process.
		signal.Notify(s)
		handler <- &signalHandler{
			signals: s,
		}
	}()
	return handler
}

// exit models a process exit status with the pid and
// exit status.
type exit struct {
	pid    int
	status int
}

type signalHandler struct {
	signals chan os.Signal
}

// forward handles the main signal event loop forwarding, resizing, or reaping depending
// on the signal received.
func (h *signalHandler) forward(process *libcontainer.Process, tty *tty) (int, error) {
	// make sure we know the pid of our main process so that we can return
	// after it dies.
	pid1, err := process.Pid()
	if err != nil {
		return -1, err
	}

	// Perform the initial tty resize. Always ignore errors resizing because
	// stdout might have disappeared (due to races with when SIGHUP is sent).
	_ = tty.resize()
	// Handle and forward signals.
	for s := range h.signals {
		switch s {
		case unix.SIGWINCH:
			// Ignore errors resizing, as above.
			_ = tty.resize()
		case unix.SIGCHLD:
			exits, err := h.reap()
			if err != nil {
				logrus.Error(err)
			}
			for _, e := range exits {
				logrus.WithFields(logrus.Fields{
					"pid":    e.pid,
					"status": e.status,
				}).Debug("process exited")
				if e.pid == pid1 {
					// call Wait() on the process even though we already have the exit
					// status because we must ensure that any of the go specific process
					// fun such as flushing pipes are complete before we return.
					_, _ = process.Wait()
					return e.status, nil
				}
			}
		case unix.SIGURG:
			// SIGURG is used by go runtime for async preemptive
			// scheduling, so runc receives it from time to time,
			// and it should not be forwarded to the container.
			// Do nothing.
		default:
			us := s.(unix.Signal)
			logrus.Debugf("forwarding signal %d (%s) to %d", int(us), unix.SignalName(us), pid1)
			if err := process.Signal(s); err != nil {
				logrus.Error(err)
			}
		}
	}
	return -1, nil
}

// reap runs wait4 in a loop until we have finished processing any existing exits
// then returns all exits to the main event loop for further processing.
func (h *signalHandler) reap() (exits []exit, err error) {
	var (
		ws  unix.WaitStatus
		rus unix.Rusage
	)
	for {
		pid, err := unix.Wait4(-1, &ws, unix.WNOHANG, &rus)
		if err != nil {
			if err == unix.ECHILD {
				return exits, nil
			}
			return nil, err
		}
		if pid <= 0 {
			return exits, nil
		}
		exits = append(exits, exit{
			pid:    pid,
			status: utils.ExitStatus(ws),
		})
	}
}

package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"
)

var killCommand = &cli.Command{
	Name:  "kill",
	Usage: "kill sends the specified signal (default: SIGTERM) to the container's init process",
	ArgsUsage: `<container-id> [signal]

Where "<container-id>" is the name for the instance of the container and
"[signal]" is the signal to be sent to the init process.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runc kill ubuntu01 KILL`,
	// Stop parsing flags after the first positional argument (container ID).
	StopOnNthArg: intPtr(1),
	// Disable comma as separator for slice flags.
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "(obsoleted, do not use)",
			Hidden:  true,
		},
	},
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, minArgs); err != nil {
			return err
		}
		if err := checkArgs(cmd, 2, maxArgs); err != nil {
			return err
		}
		container, err := getContainer(cmd)
		if err != nil {
			return err
		}

		sigstr := cmd.Args().Get(1)
		if sigstr == "" {
			sigstr = "SIGTERM"
		}

		signal, err := parseSignal(sigstr)
		if err != nil {
			return err
		}
		err = container.Signal(signal)
		if errors.Is(err, libcontainer.ErrNotRunning) && cmd.Bool("all") {
			err = nil
		}
		return err
	},
}

func parseSignal(rawSignal string) (unix.Signal, error) {
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return unix.Signal(s), nil
	}
	sig := strings.ToUpper(rawSignal)
	if !strings.HasPrefix(sig, "SIG") {
		sig = "SIG" + sig
	}
	signal := unix.SignalNum(sig)
	if signal == 0 {
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}

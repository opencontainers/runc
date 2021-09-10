package main

import (
	"fmt"
	"strconv"

	"github.com/moby/sys/signal"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill sends the specified signal (default: SIGTERM) to the container's init process",
	ArgsUsage: `<container-id> [signal]

Where "<container-id>" is the name for the instance of the container and
"[signal]" is the signal to be sent to the init process.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runc kill ubuntu01 KILL`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "all, a",
			Usage: "send the specified signal to all processes inside the container",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}
		if err := checkArgs(context, 2, maxArgs); err != nil {
			return err
		}
		container, err := getContainer(context)
		if err != nil {
			return err
		}

		sigstr := context.Args().Get(1)
		if sigstr == "" {
			sigstr = "SIGTERM"
		}

		signal, err := parseSignal(sigstr)
		if err != nil {
			return err
		}
		return container.Signal(signal, context.Bool("all"))
	},
}

func parseSignal(rawSignal string) (unix.Signal, error) {
	// allow 0 as a valid signal
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return unix.Signal(s), nil
	}
	signal, err := signal.ParseSignal(rawSignal)
	if err != nil {
		return -1, err
	}
	if signal == 0 {
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}

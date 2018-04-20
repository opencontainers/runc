package command

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/api"
	"github.com/urfave/cli"
)

func NewKillCommand(apiNew APINew) cli.Command {
	return cli.Command{
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
			if err := CheckArgs(context, 1, MinArgs); err != nil {
				return err
			}
			if err := CheckArgs(context, 2, MaxArgs); err != nil {
				return err
			}
			id, err := GetID(context)
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
			a, err := apiNew(NewGlobalConfig(context))
			if err != nil {
				return err
			}
			return a.Kill(id, signal, api.KillOpts{
				All: context.Bool("all"),
			})
		},
	}
}

func parseSignal(rawSignal string) (syscall.Signal, error) {
	s, err := strconv.Atoi(rawSignal)
	if err == nil {
		return syscall.Signal(s), nil
	}
	signal, ok := signalMap[strings.TrimPrefix(strings.ToUpper(rawSignal), "SIG")]
	if !ok {
		return -1, fmt.Errorf("unknown signal %q", rawSignal)
	}
	return signal, nil
}

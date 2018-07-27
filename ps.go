// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/containers/psgo"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/urfave/cli"
)

var psCommand = cli.Command{
	Name:      "ps",
	Usage:     "ps displays the processes running inside a container",
	ArgsUsage: `<container-id> [format descriptors]`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "table",
			Usage: `select one of: ` + formatOptions,
		},
		cli.BoolFlag{
			Name:  "list-descriptors",
			Usage: "print the list of supported format descriptors",
		},
	},
	Action: func(context *cli.Context) error {
		if context.Bool("list-descriptors") {
			fmt.Println(strings.Join(psgo.ListDescriptors(), ", "))
			return nil
		}
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}
		// XXX: Currently not supported with rootless containers.
		rootless, err := isRootless(context)
		if err != nil {
			return err
		}
		if rootless {
			return fmt.Errorf("runc ps requires root")
		}

		container, err := getContainer(context)
		if err != nil {
			return err
		}

		status, err := container.Status()
		if err != nil {
			return fmt.Errorf("%s: %v\n", err, status)
		}
		if status != libcontainer.Running {
			return fmt.Errorf("Container not running (status: %s)", status)
		}

		switch context.String("format") {
		case "table":
		case "json":
			pids, err := container.Processes()
			if err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(pids)
		default:
			return fmt.Errorf("invalid format option")
		}

		state, err := container.State()
		if err != nil {
			return fmt.Errorf("%s: %v\n", err, state)
		}
		initPid := state.BaseState.InitProcessPid

		// [1:] is to remove command name, ex:
		// context.Args(): [container_id ps_arg1 ps_arg2 ...]
		// psArgs:         [ps_arg1 ps_arg2 ...]
		//
		psArgs := context.Args()[1:]
		if len(psArgs) == 0 {
			psArgs = []string{"user", "pid", "ppid", "pcpu", "etime", "tty", "time", "comm"}
		}

		data, err := psgo.JoinNamespaceAndProcessInfo(strconv.Itoa(initPid), psArgs)
		if err != nil {
			return fmt.Errorf("%s: %s", err, data)
		}

		tw := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)
		for _, d := range data {
			fmt.Fprintln(tw, strings.Join(d, "\t"))
		}
		tw.Flush()
		return nil
	},
	SkipArgReorder: true,
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/urfave/cli/v3"
)

var psCommand = &cli.Command{
	Name:      "ps",
	Usage:     "ps displays the processes running inside a container",
	ArgsUsage: `<container-id> [ps options]`,
	// Stop parsing flags after the first positional argument (the container ID).
	// This allows passing flags like -aux to the underlying ps command.
	StopOnNthArg: intPtr(1),
	// Disable comma as separator for slice flags.
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "format",
			Aliases: []string{"f"},
			Value:   "table",
			Usage:   `select one of: ` + formatOptions,
		},
	},
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, minArgs); err != nil {
			return err
		}

		container, err := getContainer(cmd)
		if err != nil {
			return err
		}

		pids, err := container.Processes()
		if err != nil {
			maybeLogCgroupWarning("ps", err)
			return err
		}

		switch cmd.String("format") {
		case "table":
		case "json":
			return json.NewEncoder(os.Stdout).Encode(pids)
		default:
			return errors.New("invalid format option")
		}

		// [1:] is to remove command name, ex:
		// cmd.Args(): [container_id ps_arg1 ps_arg2 ...]
		// psArgs:         [ps_arg1 ps_arg2 ...]
		//
		psArgs := cmd.Args().Slice()[1:]
		if len(psArgs) == 0 {
			psArgs = []string{"-ef"}
		}

		cmdExec := exec.Command("ps", psArgs...)
		output, err := cmdExec.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, output)
		}

		lines := strings.Split(string(output), "\n")
		pidIndex, err := getPidIndex(lines[0])
		if err != nil {
			return err
		}

		fmt.Println(lines[0])
		for _, line := range lines[1:] {
			if len(line) == 0 {
				continue
			}
			fields := strings.Fields(line)
			p, err := strconv.Atoi(fields[pidIndex])
			if err != nil {
				return fmt.Errorf("unable to parse pid: %w", err)
			}

			if slices.Contains(pids, p) {
				fmt.Println(line)
			}
		}
		return nil
	},
}

func getPidIndex(title string) (int, error) {
	titles := strings.Fields(title)

	pidIndex := -1
	for i, name := range titles {
		if name == "PID" {
			return i, nil
		}
	}

	return pidIndex, errors.New("couldn't find PID field in ps output")
}

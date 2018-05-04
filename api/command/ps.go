package command

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/urfave/cli"
)

func NewPSCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:      "ps",
		Usage:     "ps displays the processes running inside a container",
		ArgsUsage: `<container-id> [ps options]`,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "format, f",
				Value: "table",
				Usage: `select one of: ` + formatOptions,
			},
		},
		Action: func(ctx *cli.Context) error {
			if err := CheckArgs(ctx, 1, MinArgs); err != nil {
				return err
			}
			id, err := GetID(ctx)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(ctx))
			if err != nil {
				return err
			}
			pids, err := a.PS(context.Background(), id)
			if err != nil {
				return err
			}
			switch ctx.String("format") {
			case "table":
			case "json":
				return json.NewEncoder(os.Stdout).Encode(pids)
			default:
				return fmt.Errorf("invalid format option")
			}

			// [1:] is to remove command name, ex:
			// ctx.Args(): [containet_id ps_arg1 ps_arg2 ...]
			// psArgs:         [ps_arg1 ps_arg2 ...]
			//
			psArgs := ctx.Args()[1:]
			if len(psArgs) == 0 {
				psArgs = []string{"-ef"}
			}

			cmd := exec.Command("ps", psArgs...)
			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%s: %s", err, output)
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
					return fmt.Errorf("unexpected pid '%s': %s", fields[pidIndex], err)
				}

				for _, pid := range pids {
					if pid == p {
						fmt.Println(line)
						break
					}
				}
			}
			return nil
		},
		SkipArgReorder: true,
	}
}

func getPidIndex(title string) (int, error) {
	var (
		titles   = strings.Fields(title)
		pidIndex = -1
	)
	for i, name := range titles {
		if name == "PID" {
			return i, nil
		}
	}
	return pidIndex, fmt.Errorf("couldn't find PID field in ps output")
}

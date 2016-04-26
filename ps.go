// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/codegangsta/cli"
)

var psCommand = cli.Command{
	Name:      "ps",
	Usage:     "ps displays the processes running inside a container",
	ArgsUsage: `<container-id> <ps options>`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "",
			Usage: `select one of: ` + formatOptions + `.

The default format is table.  The following will output the processes of a container
in json format:

    # runc ps -f json`,
		},
	},
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(err)
		}

		if context.String("format") == "json" {
			pids, err := container.Processes()
			if err != nil {
				fatal(err)
			}
			if err := json.NewEncoder(os.Stdout).Encode(pids); err != nil {
				fatal(err)
			}
			return
		}

		psArgs := context.Args().Get(1)
		if psArgs == "" {
			psArgs = "-ef"
		}

		output, err := exec.Command("ps", strings.Split(psArgs, " ")...).Output()
		if err != nil {
			fatal(err)
		}

		lines := strings.Split(string(output), "\n")
		pidIndex, err := getPidIndex(lines[0])
		if err != nil {
			fatal(err)
		}

		pids, err := container.Processes()
		if err != nil {
			fatal(err)
		}
		fmt.Println(lines[0])
		for _, line := range lines[1:] {
			if len(line) == 0 {
				continue
			}
			fields := strings.Fields(line)
			p, err := strconv.Atoi(fields[pidIndex])
			if err != nil {
				fatal(fmt.Errorf("unexpected pid '%s': %s", fields[pidIndex], err))
			}

			for _, pid := range pids {
				if pid == p {
					fmt.Println(line)
					break
				}
			}
		}
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

	return pidIndex, fmt.Errorf("couldn't find PID field in ps output")
}

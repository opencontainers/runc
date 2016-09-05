// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// psCmd represents the ps command
var psCmd = &cobra.Command{
	Short: "ps displays the processes running inside a container",
	Use:   "ps [command options] <container-id> [ps options]",
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		container, err := getContainer(flags, args)
		if err != nil {
			return err
		}

		pids, err := container.Processes()
		if err != nil {
			return err
		}

		if format, _ := flags.GetString("format"); format == "json" {
			if err := json.NewEncoder(os.Stdout).Encode(pids); err != nil {
				return err
			}
			return nil
		}

		// [1:] is to remove command name, ex:
		// context.Args(): [containet_id ps_arg1 ps_arg2 ...]
		// psArgs:         [ps_arg1 ps_arg2 ...]
		//
		psArgs := args[1:]
		if len(psArgs) == 0 {
			psArgs = []string{"-ef"}
		}

		output, err := exec.Command("ps", psArgs...).CombinedOutput()
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
}

func init() {
	flags := psCmd.Flags()

	flags.SetInterspersed(false)
	flags.StringP("format", "f", "table", "select one of: table or json")
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

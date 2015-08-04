// +build linux

package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"strconv"
	"strings"
	"syscall"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a container",
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(fmt.Errorf("%s", err))
		}
		sigStr := context.Args().First()
		state, err := container.Status()
		if err != nil {
			fatal(fmt.Errorf("Container not running %d", state))
			// return here
		}
		var sig uint64
		sigN, err := strconv.ParseUint(sigStr, 10, 5)
		if err != nil {
			//The signal is not a number, treat it as a string (either like
			//KILL" or like "SIGKILL")
			syscallSig, ok := SignalMap[strings.TrimPrefix(sigStr, "SIG")]
			if !ok {
				fatal(fmt.Errorf("Invalid Signal: %s", sigStr))
			}
			sig = uint64(syscallSig)
			errVal := container.Signal(syscall.Signal(sig))
			if errVal != nil {
				fatal(fmt.Errorf("%s", errVal))
			}
		} else {
			sig = sigN
			errVar := container.Signal(syscall.Signal(sig))
			if errVar != nil {
				fatal(fmt.Errorf("%s", errVar))
			}

		}
		if sig == 0 {
			fatal(fmt.Errorf("Invalid signal: %s", sigStr))
		}
	},
}

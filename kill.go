// +build linux

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
)

var SignalMap = map[string]syscall.Signal{
	"ABRT":   syscall.SIGABRT,
	"ALRM":   syscall.SIGALRM,
	"BUS":    syscall.SIGBUS,
	"CHLD":   syscall.SIGCHLD,
	"CLD":    syscall.SIGCLD,
	"CONT":   syscall.SIGCONT,
	"FPE":    syscall.SIGFPE,
	"HUP":    syscall.SIGHUP,
	"ILL":    syscall.SIGILL,
	"INT":    syscall.SIGINT,
	"IO":     syscall.SIGIO,
	"IOT":    syscall.SIGIOT,
	"KILL":   syscall.SIGKILL,
	"PIPE":   syscall.SIGPIPE,
	"POLL":   syscall.SIGPOLL,
	"PROF":   syscall.SIGPROF,
	"PWR":    syscall.SIGPWR,
	"QUIT":   syscall.SIGQUIT,
	"SEGV":   syscall.SIGSEGV,
	"STKFLT": syscall.SIGSTKFLT,
	"STOP":   syscall.SIGSTOP,
	"SYS":    syscall.SIGSYS,
	"TERM":   syscall.SIGTERM,
	"TRAP":   syscall.SIGTRAP,
	"TSTP":   syscall.SIGTSTP,
	"TTIN":   syscall.SIGTTIN,
	"TTOU":   syscall.SIGTTOU,
	"UNUSED": syscall.SIGUNUSED,
	"URG":    syscall.SIGURG,
	"USR1":   syscall.SIGUSR1,
	"USR2":   syscall.SIGUSR2,
	"VTALRM": syscall.SIGVTALRM,
	"WINCH":  syscall.SIGWINCH,
	"XCPU":   syscall.SIGXCPU,
	"XFSZ":   syscall.SIGXFSZ,
}

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill a container",
	Action: func(context *cli.Context) {
		container, err := getContainer(context)
		if err != nil {
			fatal(fmt.Errorf("%s", err))
		}
		sigStr := context.Args().First()
		var sig uint64
		sigN, err := strconv.ParseUint(sigStr, 10, 5)
		if err != nil {
			//The signal is not a number, treat it as a string (either like
			//KILL" or like "SIGKILL")
			syscallSig, ok := SignalMap[strings.TrimPrefix(sigStr, "SIG")]
			if !ok {
				fatal(fmt.Errorf("Invalid signal: %s", sigStr))
			}
			sig = uint64(syscallSig)
		} else {
			sig = sigN
		}

		if sig == 0 {
			fatal(fmt.Errorf("Invalid signal: %s", sigStr))
		}

		state, err := container.Status()
		if err != nil {
			fatal(fmt.Errorf("Container not running %d", state))
			// return here
		}
		if sig == 0 || syscall.Signal(sig) == syscall.SIGKILL {
			if err := Kill(container); err != nil {
				fatal(fmt.Errorf("Can not kill the container"))
			}
		} else {
			// Otherwise, just send the requested signal
			if err := KillSig(container, int(sig)); err != nil {
				fatal(fmt.Errorf("Can not kill the container"))
			}
		}

	},
}

func Kill(container libcontainer.Container) error {

	processes, err := container.Processes()
	if err != nil {
		fatal(fmt.Errorf("Can not kill the container %d", err))
		return err
	}
	syscall.Kill(processes[0], 9)
	return nil

}

func KillSig(container libcontainer.Container, sig int) error {
	processes, err := container.Processes()
	if err != nil {
		fatal(fmt.Errorf("Can not kill the container %d", err))
		return err
	}
	syscall.Kill(processes[0], syscall.Signal(sig))
	return nil
}

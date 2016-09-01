// +build linux

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/urfave/cli"
)

var signalMap = map[string]syscall.Signal{
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
	Usage: "kill sends the specified signal (default: SIGTERM) to any of the container's processes (default: init process)",
	ArgsUsage: `<container-id> <signal>

Where "<container-id>" is the name for the instance of the container and
"<signal>" is the signal to be sent to the process of the container.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:
	 
       # runc kill ubuntu01 KILL`,
	Flags: []cli.Flag{
		cli.IntFlag{
			Name:  "pid, p",
			Usage: "specify the pid to which process the signal would be sent (default: init process)",
		},
	},
	Action: func(context *cli.Context) error {
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

		pid := context.Int("pid")
		if pid == 0 {
			if err := container.Signal(signal); err != nil {
				return err
			}
			return nil
		}
		if err := sendSignal(container, pid, signal); err != nil {
			return err
		}
		return nil
	},
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

func sendSignal(container libcontainer.Container, pid int, signal syscall.Signal) error {
	if os.Args[0] == "enter_pid_ns_kill" {
		return syscall.Kill(pid, signal)
	}

	pids, err := container.Processes()
	if err != nil || len(pids) == 0 {
		return err
	}
	ns := fmt.Sprintf("/proc/%d/ns/pid", pids[0])
	fd, err := syscall.Open(ns, syscall.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("open /proc/%d/ns/pid %v", pid, err)
	}
	defer syscall.Close(fd)
	err = system.Setns(uintptr(fd), 0)
	if err != nil {
		return fmt.Errorf("setns on ipc %v", err)
	}

	args := []string{"enter_pid_ns_kill"}
	args = append(args, os.Args[1:]...)
	cmd := exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	return cmd.Run()
}

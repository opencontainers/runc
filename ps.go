// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/system"
	"github.com/urfave/cli"
)

var psCommand = cli.Command{
	Name:      "ps",
	Usage:     "ps displays the processes running inside a container",
	ArgsUsage: `<container-id> [-- ps options]`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "format, f",
			Value: "",
			Usage: `select one of: ` + formatOptions,
		},
	},
	Action: func(context *cli.Context) error {
		container, err := getContainer(context)
		if err != nil {
			return err
		}
		pids, err := container.Processes()
		if err != nil || len(pids) == 0 {
			return err
		}
		if context.String("format") == "json" {
			if err := json.NewEncoder(os.Stdout).Encode(pids); err != nil {
				return err
			}
			return nil
		}
		return ps(container, pids, context.Args()[1:])
	},
}

func ps(container libcontainer.Container, pids []int, psArgs []string) error {
	cmdPs := "enter_pid_ns_ps"
	if os.Args[0] == cmdPs {
		err := syscall.Mount("ps", "/proc", "proc", 0, "")
		if err != nil {
			return err
		}
		defer syscall.Unmount("/proc", 0)

		// [1:] is to remove command name, ex:
		// context.Args(): [containet_id ps_arg1 ps_arg2 ...]
		// psArgs:         [ps_arg1 ps_arg2 ...]
		//
		if len(psArgs) > 0 && psArgs[0] == "--" {
			psArgs = psArgs[1:]
		}

		if len(psArgs) == 0 {
			psArgs = []string{"-ef"}
		}

		output, err := exec.Command("ps", psArgs...).Output()
		if err != nil {
			return err
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
			if fields[pidIndex] != cmdPs {
				fmt.Println(line)
			}
		}
		return nil
	}

	ns := fmt.Sprintf("/proc/%d/ns/pid", pids[0])
	fd, err := syscall.Open(ns, syscall.O_RDONLY, 0644)
	if err != nil {
		return fmt.Errorf("open /proc/%d/ns/pid %v", pids[0], err)
	}
	defer syscall.Close(fd)
	err = system.Setns(uintptr(fd), 0)
	if err != nil {
		return fmt.Errorf("setns on ipc %v", err)
	}

	args := []string{cmdPs}
	args = append(args, os.Args[1:]...)
	cmd := exec.Cmd{
		Path:   "/proc/self/exe",
		Args:   args,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWNS,
	}

	return cmd.Run()
}

func getPidIndex(title string) (int, error) {
	titles := strings.Fields(title)

	pidIndex := -1
	for i, name := range titles {
		if name == "CMD" || name == "COMMAND" {
			return i, nil
		}
	}

	return pidIndex, fmt.Errorf("couldn't find PID field in ps output")
}

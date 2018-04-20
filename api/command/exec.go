package command

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

func NewExecCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "exec",
		Usage: "execute new process inside the container",
		ArgsUsage: `<container-id> <command> [command options]  || -p process.json <container-id>

Where "<container-id>" is the name for the instance of the container and
"<command>" is the command to be executed in the container.
"<command>" can't be empty unless a "-p" flag provided.

EXAMPLE:
For example, if the container is configured to run the linux ps command the
following will output a list of processes running in the container:

       # runc exec <container-id> ps`,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "console-socket",
				Usage: "path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal",
			},
			cli.StringFlag{
				Name:  "cwd",
				Usage: "current working directory in the container",
			},
			cli.StringSliceFlag{
				Name:  "env, e",
				Usage: "set environment variables",
			},
			cli.BoolFlag{
				Name:  "tty, t",
				Usage: "allocate a pseudo-TTY",
			},
			cli.StringFlag{
				Name:  "user, u",
				Usage: "UID (format: <uid>[:<gid>])",
			},
			cli.Int64SliceFlag{
				Name:  "additional-gids, g",
				Usage: "additional gids",
			},
			cli.StringFlag{
				Name:  "process, p",
				Usage: "path to the process.json",
			},
			cli.BoolFlag{
				Name:  "detach,d",
				Usage: "detach from the container's process",
			},
			cli.StringFlag{
				Name:  "pid-file",
				Value: "",
				Usage: "specify the file to write the process id to",
			},
			cli.StringFlag{
				Name:  "process-label",
				Usage: "set the asm process label for the process commonly used with selinux",
			},
			cli.StringFlag{
				Name:  "apparmor",
				Usage: "set the apparmor profile for the process",
			},
			cli.BoolFlag{
				Name:  "no-new-privs",
				Usage: "set the no new privileges value for the process",
			},
			cli.StringSliceFlag{
				Name:  "cap, c",
				Value: &cli.StringSlice{},
				Usage: "add a capability to the bounding set for the process",
			},
			cli.BoolFlag{
				Name:   "no-subreaper",
				Usage:  "disable the use of the subreaper used to reap reparented processes",
				Hidden: true,
			},
		},
		Action: func(context *cli.Context) error {
			if err := CheckArgs(context, 1, MinArgs); err != nil {
				return err
			}
			id, err := GetID(context)
			if err != nil {
				return err
			}
			pidFile, err := revisePidFile(context)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(context))
			if err != nil {
				return err
			}
			path := context.String("process")
			if path == "" && len(context.Args()) == 1 {
				return fmt.Errorf("process args cannot be empty")
			}
			state, err := a.State(id)
			if err != nil {
				return err
			}
			process, err := getProcess(context, state.Bundle)
			if err != nil {
				return err
			}
			result, err := a.Exec(id, api.ExecOpts{
				PidFile:       pidFile,
				Detach:        context.Bool("detach"),
				Process:       process,
				ConsoleSocket: context.String("console-socket"),
			})
			if err != nil {
				return err
			}
			os.Exit(result.Status)
			return nil
		},
		SkipArgReorder: true,
	}
}

func getProcess(context *cli.Context, bundle string) (*specs.Process, error) {
	if path := context.String("process"); path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		var p specs.Process
		if err := json.NewDecoder(f).Decode(&p); err != nil {
			return nil, err
		}
		return &p, validateProcessSpec(&p)
	}

	// process via cli flags
	if err := os.Chdir(bundle); err != nil {
		return nil, err
	}
	spec, err := LoadSpec("config.json")
	if err != nil {
		return nil, err
	}
	p := spec.Process
	p.Args = context.Args()[1:]
	// override the cwd, if passed
	if context.String("cwd") != "" {
		p.Cwd = context.String("cwd")
	}
	if ap := context.String("apparmor"); ap != "" {
		p.ApparmorProfile = ap
	}
	if l := context.String("process-label"); l != "" {
		p.SelinuxLabel = l
	}
	if caps := context.StringSlice("cap"); len(caps) > 0 {
		for _, c := range caps {
			p.Capabilities.Bounding = append(p.Capabilities.Bounding, c)
			p.Capabilities.Inheritable = append(p.Capabilities.Inheritable, c)
			p.Capabilities.Effective = append(p.Capabilities.Effective, c)
			p.Capabilities.Permitted = append(p.Capabilities.Permitted, c)
			p.Capabilities.Ambient = append(p.Capabilities.Ambient, c)
		}
	}
	// append the passed env variables
	p.Env = append(p.Env, context.StringSlice("env")...)

	// set the tty
	if context.IsSet("tty") {
		p.Terminal = context.Bool("tty")
	}
	if context.IsSet("no-new-privs") {
		p.NoNewPrivileges = context.Bool("no-new-privs")
	}
	// override the user, if passed
	if context.String("user") != "" {
		u := strings.SplitN(context.String("user"), ":", 2)
		if len(u) > 1 {
			gid, err := strconv.Atoi(u[1])
			if err != nil {
				return nil, fmt.Errorf("parsing %s as int for gid failed: %v", u[1], err)
			}
			p.User.GID = uint32(gid)
		}
		uid, err := strconv.Atoi(u[0])
		if err != nil {
			return nil, fmt.Errorf("parsing %s as int for uid failed: %v", u[0], err)
		}
		p.User.UID = uint32(uid)
	}
	for _, gid := range context.Int64Slice("additional-gids") {
		if gid < 0 {
			return nil, fmt.Errorf("additional-gids must be a positive number %d", gid)
		}
		p.User.AdditionalGids = append(p.User.AdditionalGids, uint32(gid))
	}
	return p, nil
}

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli/v3"
)

var execCommand = &cli.Command{
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
	// Stop parsing flags after the first positional argument (command)
	// This allows passing flags like -c to the command being executed
	StopOnNthArg: intPtr(1),
	// Disable comma as separator for slice flags
	// This allows cgroup controller lists like "cpu,cpuacct:subcpu".
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "console-socket",
			Usage: "path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal",
		},
		&cli.StringFlag{
			Name:  "pidfd-socket",
			Usage: "path to an AF_UNIX socket which will receive a file descriptor referencing the exec process",
		},
		&cli.StringFlag{
			Name:  "cwd",
			Usage: "current working directory in the container",
		},
		&cli.StringSliceFlag{
			Name:    "env",
			Aliases: []string{"e"},
			Usage:   "set environment variables",
		},
		&cli.BoolFlag{
			Name:    "tty",
			Aliases: []string{"t"},
			Usage:   "allocate a pseudo-TTY",
		},
		&cli.StringFlag{
			Name:    "user",
			Aliases: []string{"u"},
			Usage:   "UID (format: <uid>[:<gid>])",
		},
		&cli.Int64SliceFlag{
			Name:    "additional-gids",
			Aliases: []string{"g"},
			Usage:   "additional gids",
		},
		&cli.StringFlag{
			Name:    "process",
			Aliases: []string{"p"},
			Usage:   "path to the process.json",
		},
		&cli.BoolFlag{
			Name:    "detach",
			Aliases: []string{"d"},
			Usage:   "detach from the container's process",
		},
		&cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		&cli.StringFlag{
			Name:  "process-label",
			Usage: "set the asm process label for the process commonly used with selinux",
		},
		&cli.StringFlag{
			Name:  "apparmor",
			Usage: "set the apparmor profile for the process",
		},
		&cli.BoolFlag{
			Name:  "no-new-privs",
			Usage: "set the no new privileges value for the process",
		},
		&cli.StringSliceFlag{
			Name:    "cap",
			Aliases: []string{"c"},
			Value:   []string{},
			Usage:   "add a capability to the bounding set for the process",
		},
		&cli.IntFlag{
			Name:  "preserve-fds",
			Usage: "Pass N additional file descriptors to the container (stdio + $LISTEN_FDS + N in total)",
		},
		&cli.StringSliceFlag{
			Name:  "cgroup",
			Usage: "run the process in an (existing) sub-cgroup(s). Format is [<controller>:]<cgroup>.",
		},
		&cli.BoolFlag{
			Name:  "ignore-paused",
			Usage: "allow exec in a paused container",
		},
	},
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, minArgs); err != nil {
			return err
		}
		if err := revisePidFile(cmd); err != nil {
			return err
		}
		status, err := execProcess(cmd)
		if err == nil {
			os.Exit(status)
		}
		fatalWithCode(fmt.Errorf("exec failed: %w", err), 255)
		return nil // to satisfy the linter
	},
}

// getSubCgroupPaths parses --cgroup arguments, which can either be
//   - a single "path" argument (for cgroup v2);
//   - one or more controller[,controller[,...]]:path arguments (for cgroup v1).
//
// Returns a controller to path map. For cgroup v2, it's a single entity map
// with empty controller value.
func getSubCgroupPaths(args []string) (map[string]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	paths := make(map[string]string, len(args))
	for _, c := range args {
		// Split into controller:path.
		if ctr, path, ok := strings.Cut(c, ":"); ok {
			// There may be a few comma-separated controllers.
			for ctrl := range strings.SplitSeq(ctr, ",") {
				if ctrl == "" {
					return nil, fmt.Errorf("invalid --cgroup argument: %s (empty <controller> prefix)", c)
				}
				if _, ok := paths[ctrl]; ok {
					return nil, fmt.Errorf("invalid --cgroup argument(s): controller %s specified multiple times", ctrl)
				}
				paths[ctrl] = path
			}
		} else {
			// No "controller:" prefix (cgroup v2, a single path).
			if len(args) != 1 {
				return nil, fmt.Errorf("invalid --cgroup argument: %s (missing <controller>: prefix)", c)
			}
			paths[""] = c
		}
	}
	return paths, nil
}

func execProcess(cmd *cli.Command) (int, error) {
	container, err := getContainer(cmd)
	if err != nil {
		return -1, err
	}
	status, err := container.Status()
	if err != nil {
		return -1, err
	}
	if status == libcontainer.Stopped {
		return -1, errors.New("cannot exec in a stopped container")
	}
	if status == libcontainer.Paused && !cmd.Bool("ignore-paused") {
		return -1, errors.New("cannot exec in a paused container (use --ignore-paused to override)")
	}
	p, err := getProcess(cmd, container)
	if err != nil {
		return -1, err
	}

	cgPaths, err := getSubCgroupPaths(cmd.StringSlice("cgroup"))
	if err != nil {
		return -1, err
	}

	r := &runner{
		enableSubreaper: false,
		shouldDestroy:   false,
		container:       container,
		consoleSocket:   cmd.String("console-socket"),
		pidfdSocket:     cmd.String("pidfd-socket"),
		detach:          cmd.Bool("detach"),
		pidFile:         cmd.String("pid-file"),
		action:          CT_ACT_RUN,
		init:            false,
		preserveFDs:     cmd.Int("preserve-fds"),
		subCgroupPaths:  cgPaths,
	}
	return r.run(p)
}

func getProcess(cmd *cli.Command, c *libcontainer.Container) (*specs.Process, error) {
	if path := cmd.String("process"); path != "" {
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
	// Process from config.json and CLI flags.
	bundle, ok := utils.SearchLabels(c.Config().Labels, "bundle")
	if !ok {
		return nil, errors.New("bundle not found in labels")
	}
	if err := os.Chdir(bundle); err != nil {
		return nil, err
	}
	spec, err := loadSpec(specConfig)
	if err != nil {
		return nil, err
	}
	p := spec.Process
	args := cmd.Args().Slice()
	if len(args) < 2 {
		return nil, errors.New("exec args cannot be empty")
	}
	p.Args = args[1:]
	// Override the cwd, if passed.
	if cwd := cmd.String("cwd"); cwd != "" {
		p.Cwd = cwd
	}
	if ap := cmd.String("apparmor"); ap != "" {
		p.ApparmorProfile = ap
	}
	if l := cmd.String("process-label"); l != "" {
		p.SelinuxLabel = l
	}
	if caps := cmd.StringSlice("cap"); len(caps) > 0 {
		for _, c := range caps {
			p.Capabilities.Bounding = append(p.Capabilities.Bounding, c)
			p.Capabilities.Effective = append(p.Capabilities.Effective, c)
			p.Capabilities.Permitted = append(p.Capabilities.Permitted, c)
			// Since ambient capabilities can't be set without inherritable,
			// and runc exec --cap don't set inheritable, let's only set
			// ambient if we already have some inheritable bits set from spec.
			if p.Capabilities.Inheritable != nil {
				p.Capabilities.Ambient = append(p.Capabilities.Ambient, c)
			}
		}
	}
	// append the passed env variables
	p.Env = append(p.Env, cmd.StringSlice("env")...)

	// Always set tty to false, unless explicitly enabled from CLI.
	p.Terminal = cmd.Bool("tty")
	if cmd.IsSet("no-new-privs") {
		p.NoNewPrivileges = cmd.Bool("no-new-privs")
	}
	// Override the user, if passed.
	if user := cmd.String("user"); user != "" {
		uids, gids, ok := strings.Cut(user, ":")
		if ok {
			gid, err := strconv.Atoi(gids)
			if err != nil {
				return nil, fmt.Errorf("bad gid: %w", err)
			}
			p.User.GID = uint32(gid)
		}
		uid, err := strconv.Atoi(uids)
		if err != nil {
			return nil, fmt.Errorf("bad uid: %w", err)
		}
		p.User.UID = uint32(uid)
	}
	for _, gid := range cmd.Int64Slice("additional-gids") {
		if gid < 0 {
			return nil, fmt.Errorf("additional-gids must be a positive number %d", gid)
		}
		p.User.AdditionalGids = append(p.User.AdditionalGids, uint32(gid))
	}
	return p, validateProcessSpec(p)
}

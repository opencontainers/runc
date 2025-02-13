package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

var execCommand = cli.Command{
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
			Name:  "pidfd-socket",
			Usage: "path to an AF_UNIX socket which will receive a file descriptor referencing the exec process",
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
		cli.IntFlag{
			Name:  "preserve-fds",
			Usage: "Pass N additional file descriptors to the container (stdio + $LISTEN_FDS + N in total)",
		},
		cli.StringSliceFlag{
			Name:  "cgroup",
			Usage: "run the process in an (existing) sub-cgroup(s). Format is [<controller>:]<cgroup>.",
		},
		cli.BoolFlag{
			Name:  "ignore-paused",
			Usage: "allow exec in a paused container",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, minArgs); err != nil {
			return err
		}
		if err := revisePidFile(context); err != nil {
			return err
		}
		status, err := execProcess(context)
		if err == nil {
			os.Exit(status)
		}
		fatalWithCode(fmt.Errorf("exec failed: %w", err), 255)
		return nil // to satisfy the linter
	},
	SkipArgReorder: true,
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
			for _, ctrl := range strings.Split(ctr, ",") {
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

func execProcess(context *cli.Context) (int, error) {
	container, err := getContainer(context)
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
	if status == libcontainer.Paused && !context.Bool("ignore-paused") {
		return -1, errors.New("cannot exec in a paused container (use --ignore-paused to override)")
	}
	p, err := getProcess(context, container)
	if err != nil {
		return -1, err
	}

	cgPaths, err := getSubCgroupPaths(context.StringSlice("cgroup"))
	if err != nil {
		return -1, err
	}

	r := &runner{
		enableSubreaper: false,
		shouldDestroy:   false,
		container:       container,
		consoleSocket:   context.String("console-socket"),
		pidfdSocket:     context.String("pidfd-socket"),
		detach:          context.Bool("detach"),
		pidFile:         context.String("pid-file"),
		action:          CT_ACT_RUN,
		init:            false,
		preserveFDs:     context.Int("preserve-fds"),
		subCgroupPaths:  cgPaths,
	}
	return r.run(p)
}

func getProcess(context *cli.Context, c *libcontainer.Container) (*specs.Process, error) {
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
	args := context.Args()
	if len(args) < 2 {
		return nil, errors.New("exec args cannot be empty")
	}
	p.Args = args[1:]
	// Override the cwd, if passed.
	if cwd := context.String("cwd"); cwd != "" {
		p.Cwd = cwd
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
	p.Env = append(p.Env, context.StringSlice("env")...)

	// Always set tty to false, unless explicitly enabled from CLI.
	p.Terminal = context.Bool("tty")
	if context.IsSet("no-new-privs") {
		p.NoNewPrivileges = context.Bool("no-new-privs")
	}
	// Override the user, if passed.
	if user := context.String("user"); user != "" {
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
	for _, gid := range context.Int64Slice("additional-gids") {
		if gid < 0 {
			return nil, fmt.Errorf("additional-gids must be a positive number %d", gid)
		}
		p.User.AdditionalGids = append(p.User.AdditionalGids, uint32(gid))
	}
	return p, validateProcessSpec(p)
}

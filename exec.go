// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/utils"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/urfave/cli"
)

var execCommand = cli.Command{
	Name:  "exec",
	Usage: "execute new process inside the container",
	ArgsUsage: `<container-id> <container command> [command options]

Where "<container-id>" is the name for the instance of the container and
"<container command>" is the command to be executed in the container.

EXAMPLE:
For example, if the container is configured to run the linux ps command the
following will output a list of processes running in the container:
	 
       # runc exec <container-id> ps`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "console",
			Usage: "specify the pty slave path for use with the container",
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
	SkipFlagParsing: true,
	SkipArgReorder:  true,
	Action: func(context *cli.Context) error {
		return CobraExecute()
	},
}

var execCmd = &cobra.Command{
	Short: "execute new process inside the container",
	Use: `exec [command options] <container-id> <container command>

Where "<container-id>" is the name for the instance of the container and
"<container command>" is the command to be executed in the container.`,
	Example: `For example, if the container is configured to run the linux ps command the
following will output a list of processes running in the container:

       # runc exec <container-id> ps`,

	RunE: func(cmd *cobra.Command, args []string) error {
		if os.Geteuid() != 0 {
			return fmt.Errorf("runc should be run as root")
		}
		status, err := execProcess(cmd.Flags(), args)
		if err == nil {
			os.Exit(status)
		}
		return fmt.Errorf("exec failed: %v", err)
	},
}

func init() {
	flags := execCmd.Flags()

	flags.SetInterspersed(false)

	flags.StringP("console", "", "", "specify the pty slave path for use with the container")
	flags.StringP("cwd", "", "", "current working directory in the container")
	flags.StringSliceP("env", "e", []string{}, "set environment variables")
	flags.BoolP("tty", "t", false, "allocate a pseudo-TTY")
	flags.StringP("user", "u", "", "UID (format: <uid>[:<gid>])")
	flags.StringP("process", "p", "", "path to the process.json")
	flags.BoolP("detach", "d", false, "detach from the container's process")
	flags.StringP("pid-file", "", "", "specify the file to write the process id to")
	flags.StringP("process-label", "", "", "set the asm process label for the process commonly used with selinux")
	flags.StringP("apparmor", "", "", "set the apparmor profile for the process")
	flags.BoolP("no-new-privs", "", false, "set the no new privileges value for the process")
	flags.StringSliceP("cap", "c", []string{}, "add a capability to the bounding set for the process")
	flags.BoolP("no-subreaper", "", false, "disable the use of the subreaper used to reap reparented processes")

	// mark "no-subreaper" as hidden
	flags.MarkHidden("no-subreaper")
}

func execProcess(flags *pflag.FlagSet, args []string) (int, error) {
	container, err := getContainerCobra(flags, args)
	if err != nil {
		return -1, err
	}
	status, err := container.Status()
	if err != nil {
		return -1, err
	}
	if status == libcontainer.Stopped {
		return -1, fmt.Errorf("cannot exec a container that has run and stopped")
	}
	path, _ := flags.GetString("process")
	if path == "" && len(args) == 1 {
		return -1, fmt.Errorf("process args cannot be empty")
	}
	detach, _ := flags.GetBool("detach")
	state, err := container.State()
	if err != nil {
		return -1, err
	}
	bundle := utils.SearchLabels(state.Config.Labels, "bundle")
	p, err := getProcess(flags, args, bundle)
	if err != nil {
		return -1, err
	}
	r := &runner{
		enableSubreaper: false,
		shouldDestroy:   false,
		container:       container,
		console:         func() string { c, _ := flags.GetString("console"); return c }(),
		detach:          detach,
		pidFile:         func() string { p, _ := flags.GetString("pid-file"); return p }(),
	}
	return r.run(p)
}

func getProcess(flags *pflag.FlagSet, args []string, bundle string) (*specs.Process, error) {
	if path, _ := flags.GetString("process"); path != "" {
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
	spec, err := loadSpec(specConfig)
	if err != nil {
		return nil, err
	}
	p := spec.Process
	p.Args = args[1:]
	// override the cwd, if passed
	if cwd, _ := flags.GetString("cwd"); cwd != "" {
		p.Cwd = cwd
	}
	if ap, _ := flags.GetString("apparmor"); ap != "" {
		p.ApparmorProfile = ap
	}
	if l, _ := flags.GetString("process-label"); l != "" {
		p.SelinuxLabel = l
	}
	if caps, _ := flags.GetStringSlice("cap"); len(caps) > 0 {
		p.Capabilities = caps
	}
	// append the passed env variables
	envs, _ := flags.GetStringSlice("env")
	for _, e := range envs {
		p.Env = append(p.Env, e)
	}
	// set the tty
	if tty, _ := flags.GetBool("tty"); tty {
		p.Terminal = tty
	}
	if v, _ := flags.GetBool("no-new-privs"); v {
		p.NoNewPrivileges = v
	}
	// override the user, if passed
	if user, _ := flags.GetString("user"); user != "" {
		u := strings.SplitN(user, ":", 2)
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
	return &p, nil
}

package command

import (
	"os"

	"github.com/opencontainers/runc/api"
	"github.com/urfave/cli"
)

func NewRunCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "run",
		Usage: "create and run a container",
		ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
		Description: `The run command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "config.json" and a root
filesystem.

The specification file includes an args parameter. The args parameter is used
to specify command(s) that get run when the container is started. To change the
command(s) that get executed on start, edit the args parameter of the spec. See
"runc spec --help" for more explanation.`,
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:  "bundle, b",
				Value: "",
				Usage: `path to the root of the bundle directory, defaults to the current directory`,
			},
			cli.StringFlag{
				Name:  "console-socket",
				Value: "",
				Usage: "path to an AF_UNIX socket which will receive a file descriptor referencing the master end of the console's pseudoterminal",
			},
			cli.BoolFlag{
				Name:  "detach, d",
				Usage: "detach from the container's process",
			},
			cli.StringFlag{
				Name:  "pid-file",
				Value: "",
				Usage: "specify the file to write the process id to",
			},
			cli.BoolFlag{
				Name:  "no-subreaper",
				Usage: "disable the use of the subreaper used to reap reparented processes",
			},
			cli.BoolFlag{
				Name:  "no-pivot",
				Usage: "do not use pivot root to jail process inside rootfs.  This should be used whenever the rootfs is on top of a ramdisk",
			},
			cli.BoolFlag{
				Name:  "no-new-keyring",
				Usage: "do not create a new session keyring for the container.  This will cause the container to inherit the calling processes session key",
			},
			cli.IntFlag{
				Name:  "preserve-fds",
				Usage: "Pass N additional file descriptors to the container (stdio + $LISTEN_FDS + N in total)",
			},
		},
		Action: func(context *cli.Context) error {
			if err := CheckArgs(context, 1, ExactArgs); err != nil {
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
			spec, err := setupSpec(context)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(context))
			if err != nil {
				return err
			}
			opts := api.CreateOpts{
				PidFile:       pidFile,
				ConsoleSocket: context.String("console-socket"),
				NoPivot:       context.Bool("no-pivot"),
				NoNewKeyring:  context.Bool("no-new-keyring"),
				PreserveFDs:   context.Int("preserve-fds"),
				Detach:        context.Bool("detach"),
				NoSubreaper:   context.Bool("no-subreaper"),
				Spec:          spec,
			}
			result, err := a.Run(id, opts)
			if err != nil {
				return err
			}
			os.Exit(result.Status)
			return nil
		},
	}
}

package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/urfave/cli"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "create a container",
	ArgsUsage: `<container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Description: `The create command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "` + specConfig + `" and a root
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
			Name:  "console",
			Value: "",
			Usage: "specify the pty slave path for use with the container",
		},
		cli.StringFlag{
			Name:  "pid-file",
			Value: "",
			Usage: "specify the file to write the process id to",
		},
		cli.BoolFlag{
			Name:  "no-pivot",
			Usage: "do not use pivot root to jail process inside rootfs.  This should be used whenever the rootfs is on top of a ramdisk",
		},
		cli.BoolFlag{
			Name:  "no-new-keyring",
			Usage: "do not create a new session keyring for the container.  This will cause the container to inherit the calling processes session key",
		},
	},
	SkipFlagParsing: true,
	Action: func(context *cli.Context) error {
		return CobraExecute()
	},
}

var createCmd = &cobra.Command{
	Short: "create a container",
	Use: `create [command options] <container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Long: `The create command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "` + specConfig + `" and a root
filesystem.

The specification file includes an args parameter. The args parameter is used
to specify command(s) that get run when the container is started. To change the
command(s) that get executed on start, edit the args parameter of the spec. See
"runc spec --help" for more explanation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()

		if len(args) != 1 {
			fmt.Printf("Incorrect Usage.\n\n")
			cmd.Usage()
			return fmt.Errorf("runc: \"create\" requires exactly one argument")
		}

		id := args[0]
		spec, err := setupSpec(flags)
		if err != nil {
			return err
		}
		status, err := startContainer(flags, id, spec, true)
		if err != nil {
			return err
		}
		// exit with the container's exit status so any external supervisor is
		// notified of the exit with the correct exit status.
		os.Exit(status)
		return nil
	},
}

func init() {
	flags := createCmd.Flags()

	flags.StringP("bundle", "b", "", "path to the root of the bundle directory, defaults to the current directory")
	flags.String("console", "", "specify the pty slave path for use with the container")
	flags.String("pid-file", "", "specify the file to write the process id to")
	flags.Bool("no-pivot", false, "do not use pivot root to jail process inside rootfs.  This should be used whenever the rootfs is on top of a ramdisk")
	flags.Bool("no-new-keyring", false, "do not create a new session keyring for the container.  This will cause the container to inherit the calling processes session key")
}

// +build linux

package main

import (
	"os"

	"github.com/spf13/cobra"
)

// default action is to start a container
var runCmd = &cobra.Command{
	Short: "create and run a container",
	Use: `run [command options] <container-id>

Where "<container-id>" is your name for the instance of the container that you
are starting. The name you provide for the container instance must be unique on
your host.`,
	Long: `The run command creates an instance of a container for a bundle. The bundle
is a directory with a specification file named "` + specConfig + `" and a root
filesystem.

The specification file includes an args parameter. The args parameter is used
to specify command(s) that get run when the container is started. To change the
command(s) that get executed on start, edit the args parameter of the spec. See
"runc spec --help" for more explanation.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		id := ""
		if len(args) >= 1 {
			id = args[0]
		}
		spec, err := setupSpec(flags)
		if err != nil {
			return err
		}
		status, err := startContainer(flags, id, spec, false)
		if err == nil {
			// exit with the container's exit status so any external supervisor is
			// notified of the exit with the correct exit status.
			os.Exit(status)
		}
		return err
	},
}

func init() {
	flags := runCmd.Flags()

	flags.StringP("bundle", "b", "", "path to the root of the bundle directory, defaults to the current directory")
	flags.String("console", "", "specify the pty slave path for use with the container")
	flags.BoolP("detach", "d", false, "detach from the container's process")
	flags.String("pid-file", "", "specify the file to write the process id to")
	flags.Bool("no-subreaper", false, "disable the use of the subreaper used to reap reparented processes")
	flags.Bool("no-pivot", false, "do not use pivot root to jail process inside rootfs.  This should be used whenever the rootfs is on top of a ramdisk")
	flags.Bool("no-new-keyring", false, "do not create a new session keyring for the container.  This will cause the container to inherit the calling processes session key")
}

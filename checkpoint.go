// +build linux

package main

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/urfave/cli"
)

var checkpointCommand = cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a running container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
checkpointed.`,
	Description: `The checkpoint command saves the state of the container instance.`,
	Flags: []cli.Flag{
		cli.StringFlag{Name: "image-path", Value: "", Usage: "path for saving criu image files"},
		cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
		cli.BoolFlag{Name: "leave-running", Usage: "leave the process running after checkpointing"},
		cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
		cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
		cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
		cli.StringFlag{Name: "page-server", Value: "", Usage: "ADDRESS:PORT of the page server"},
		cli.BoolFlag{Name: "file-locks", Usage: "handle file locks, for safety"},
		cli.StringFlag{Name: "manage-cgroups-mode", Value: "", Usage: "cgroups mode: 'soft' (default), 'full' and 'strict'"},
		cli.StringSliceFlag{Name: "empty-ns", Usage: "create a namespace, but don't restore its properies"},
	},
	SkipFlagParsing: true,
	Action: func(context *cli.Context) error {
		return CobraExecute()
	},
}

var checkpointCmd = &cobra.Command{
	Short: "checkpoint a running container",
	Use: `checkpoint [command options] <container-id>

Where "<container-id>" is the name for the instance of the container to be
checkpointed.`,
	Long: "The checkpoint command saves the state of the container instance.",
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()
		container, err := getContainerCobra(flags, args)
		if err != nil {
			return err
		}
		status, err := container.Status()
		if err != nil {
			return err
		}
		if status == libcontainer.Created {
			fatalf("Container cannot be checkpointed in created state")
		}
		defer destroy(container)
		options := criuOptions(flags)
		// these are the mandatory criu options for a container
		setPageServer(flags, options)
		setManageCgroupsMode(flags, options)
		if err := setEmptyNsMask(flags, options); err != nil {
			return err
		}
		if err := container.Checkpoint(options); err != nil {
			return err
		}
		return nil
	},
}

func init() {
	flags := checkpointCmd.Flags()

	flags.String("image-path", "", "path for saving criu image files")
	flags.String("work-path", "", "path for saving work files and logs")
	flags.Bool("leave-running", false, "leave the process running after checkpointing")
	flags.Bool("tcp-established", false, "allow open tcp connections")
	flags.Bool("ext-unix-sk", false, "allow external unix sockets")
	flags.Bool("shell-job", false, "allow shell jobs")
	flags.String("page-server", "", "ADDRESS:PORT of the page server")
	flags.Bool("file-locks", false, "handle file locks, for safety")
	flags.String("manage-cgroups-mode", "soft", "cgroups mode: 'soft', 'full' and 'strict'")
	flags.StringSlice("empty-ns", []string{}, "create a namespace, but don't restore its properies")
}

func getCheckpointImagePath(flags *pflag.FlagSet) string {
	imagePath, _ := flags.GetString("image-path")
	if imagePath == "" {
		imagePath = getDefaultImagePathCobra()
	}
	return imagePath
}

func setPageServer(flags *pflag.FlagSet, options *libcontainer.CriuOpts) {
	// xxx following criu opts are optional
	// The dump image can be sent to a criu page server
	if psOpt, _ := flags.GetString("page-server"); psOpt != "" {
		addressPort := strings.Split(psOpt, ":")
		if len(addressPort) != 2 {
			fatal(fmt.Errorf("Use --page-server ADDRESS:PORT to specify page server"))
		}
		portInt, err := strconv.Atoi(addressPort[1])
		if err != nil {
			fatal(fmt.Errorf("Invalid port number"))
		}
		options.PageServer = libcontainer.CriuPageServerInfo{
			Address: addressPort[0],
			Port:    int32(portInt),
		}
	}
}

func setManageCgroupsMode(flags *pflag.FlagSet, options *libcontainer.CriuOpts) {
	if cgOpt, _ := flags.GetString("manage-cgroups-mode"); cgOpt != "" {
		switch cgOpt {
		case "soft":
			options.ManageCgroupsMode = libcontainer.CRIU_CG_MODE_SOFT
		case "full":
			options.ManageCgroupsMode = libcontainer.CRIU_CG_MODE_FULL
		case "strict":
			options.ManageCgroupsMode = libcontainer.CRIU_CG_MODE_STRICT
		default:
			fatal(fmt.Errorf("Invalid manage cgroups mode"))
		}
	}
}

var namespaceMapping = map[specs.NamespaceType]int{
	specs.NetworkNamespace: syscall.CLONE_NEWNET,
}

func setEmptyNsMask(flags *pflag.FlagSet, options *libcontainer.CriuOpts) error {
	var nsmask int

	emptyNs, _ := flags.GetStringSlice("empty-ns")
	for _, ns := range emptyNs {
		f, exists := namespaceMapping[specs.NamespaceType(ns)]
		if !exists {
			return fmt.Errorf("namespace %q is not supported", ns)
		}
		nsmask |= f
	}

	options.EmptyNs = uint32(nsmask)
	return nil
}

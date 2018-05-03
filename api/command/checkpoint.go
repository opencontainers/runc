package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/api"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

func NewCheckpointCommand(apiNew APINew) cli.Command {
	return cli.Command{
		Name:  "checkpoint",
		Usage: "checkpoint a running container",
		ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
checkpointed.`,
		Description: `The checkpoint command saves the state of the container instance.`,
		Flags: []cli.Flag{
			cli.StringFlag{Name: "image-path", Value: "", Usage: "path for saving criu image files"},
			cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
			cli.StringFlag{Name: "parent-path", Value: "", Usage: "path for previous criu image files in pre-dump"},
			cli.BoolFlag{Name: "leave-running", Usage: "leave the process running after checkpointing"},
			cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
			cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
			cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
			cli.BoolFlag{Name: "lazy-pages", Usage: "use userfaultfd to lazily restore memory pages"},
			cli.StringFlag{Name: "status-fd", Value: "", Usage: "criu writes \\0 to this FD once lazy-pages is ready"},
			cli.StringFlag{Name: "page-server", Value: "", Usage: "ADDRESS:PORT of the page server"},
			cli.BoolFlag{Name: "file-locks", Usage: "handle file locks, for safety"},
			cli.BoolFlag{Name: "pre-dump", Usage: "dump container's memory information only, leave the container running after this"},
			cli.StringFlag{Name: "manage-cgroups-mode", Value: "", Usage: "cgroups mode: 'soft' (default), 'full' and 'strict'"},
			cli.StringSliceFlag{Name: "empty-ns", Usage: "create a namespace, but don't restore its properties"},
			cli.BoolFlag{Name: "auto-dedup", Usage: "enable auto deduplication of memory images"},
		},
		Action: func(context *cli.Context) error {
			if err := CheckArgs(context, 1, ExactArgs); err != nil {
				return err
			}
			id, err := GetID(context)
			if err != nil {
				return err
			}
			a, err := apiNew(NewGlobalConfig(context))
			if err != nil {
				return err
			}
			cr, ok := a.(api.CheckpointOperations)
			if !ok {
				return api.ErrNotImplemented
			}
			opts, err := criuOptions(context)
			if err != nil {
				return err
			}
			// these are the mandatory criu options for a container
			if err := setPageServer(context, opts); err != nil {
				return err
			}
			if err := setEmptyNsMask(context, opts); err != nil {
				return err
			}
			return cr.Checkpoint(id, *opts)
		},
	}

}

func getCheckpointImagePath(context *cli.Context) string {
	imagePath := context.String("image-path")
	if imagePath == "" {
		imagePath = getDefaultImagePath(context)
	}
	return imagePath
}

func setPageServer(context *cli.Context, options *api.CheckpointOpts) error {
	// xxx following criu opts are optional
	// The dump image can be sent to a criu page server
	if psOpt := context.String("page-server"); psOpt != "" {
		addressPort := strings.Split(psOpt, ":")
		if len(addressPort) != 2 {
			return fmt.Errorf("use --page-server ADDRESS:PORT to specify page server")
		}
		portInt, err := strconv.Atoi(addressPort[1])
		if err != nil {
			return fmt.Errorf("invalid port number")
		}
		options.PageServer = api.CriuPageServerInfo{
			Address: addressPort[0],
			Port:    int32(portInt),
		}
	}
	return nil
}

var namespaceMapping = map[specs.LinuxNamespaceType]int{
	specs.NetworkNamespace: unix.CLONE_NEWNET,
}

func setEmptyNsMask(context *cli.Context, options *api.CheckpointOpts) error {
	var nsmask int

	for _, ns := range context.StringSlice("empty-ns") {
		f, exists := namespaceMapping[specs.LinuxNamespaceType(ns)]
		if !exists {
			return fmt.Errorf("namespace %q is not supported", ns)
		}
		nsmask |= f
	}
	options.EmptyNs = uint32(nsmask)
	return nil
}

func getDefaultImagePath(context *cli.Context) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}

func criuOptions(context *cli.Context) (*api.CheckpointOpts, error) {
	imagePath := getCheckpointImagePath(context)
	if err := os.MkdirAll(imagePath, 0655); err != nil {
		return nil, err
	}
	return &api.CheckpointOpts{
		ImagesDirectory:         imagePath,
		WorkDirectory:           context.String("work-path"),
		ParentImage:             context.String("parent-path"),
		LeaveRunning:            context.Bool("leave-running"),
		TcpEstablished:          context.Bool("tcp-established"),
		ExternalUnixConnections: context.Bool("ext-unix-sk"),
		ShellJob:                context.Bool("shell-job"),
		FileLocks:               context.Bool("file-locks"),
		PreDump:                 context.Bool("pre-dump"),
		AutoDedup:               context.Bool("auto-dedup"),
		LazyPages:               context.Bool("lazy-pages"),
		StatusFd:                context.String("status-fd"),
		ManageCgroupsMode:       context.String("manage-cgroups-mode"),
	}, nil
}

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/moby/sys/userns"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer"
)

var checkpointCommand = &cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a running container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
checkpointed.`,
	Description: `The checkpoint command saves the state of the container instance.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "image-path", Value: "", Usage: "path for saving criu image files"},
		&cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
		&cli.StringFlag{Name: "parent-path", Value: "", Usage: "path for previous criu image files in pre-dump"},
		&cli.BoolFlag{Name: "leave-running", Usage: "leave the process running after checkpointing"},
		&cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
		&cli.BoolFlag{Name: "tcp-skip-in-flight", Usage: "skip in-flight tcp connections"},
		&cli.BoolFlag{Name: "link-remap", Usage: "allow one to link unlinked files back when possible"},
		&cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
		&cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
		&cli.BoolFlag{Name: "lazy-pages", Usage: "use userfaultfd to lazily restore memory pages"},
		&cli.IntFlag{Name: "status-fd", Value: -1, Usage: "criu writes \\0 to this FD once lazy-pages is ready"},
		&cli.StringFlag{Name: "page-server", Value: "", Usage: "ADDRESS:PORT of the page server"},
		&cli.BoolFlag{Name: "file-locks", Usage: "handle file locks, for safety"},
		&cli.BoolFlag{Name: "pre-dump", Usage: "dump container's memory information only, leave the container running after this"},
		&cli.StringFlag{Name: "manage-cgroups-mode", Value: "", Usage: "cgroups mode: soft|full|strict|ignore (default: soft)"},
		&cli.StringSliceFlag{Name: "empty-ns", Usage: "create a namespace, but don't restore its properties"},
		&cli.BoolFlag{Name: "auto-dedup", Usage: "enable auto deduplication of memory images"},
	},
	Action: func(_ context.Context, cmd *cli.Command) error {
		if err := checkArgs(cmd, 1, exactArgs); err != nil {
			return err
		}
		// XXX: Currently this is untested with rootless containers.
		if os.Geteuid() != 0 || userns.RunningInUserNS() {
			logrus.Warn("runc checkpoint is untested with rootless containers")
		}

		container, err := getContainer(cmd)
		if err != nil {
			return err
		}
		status, err := container.Status()
		if err != nil {
			return err
		}
		if status == libcontainer.Created || status == libcontainer.Stopped {
			return fmt.Errorf("Container cannot be checkpointed in %s state", status.String())
		}
		options, err := criuOptions(cmd)
		if err != nil {
			return err
		}

		err = container.Checkpoint(options)
		if err == nil && !options.LeaveRunning && !options.PreDump {
			// Destroy the container unless we tell CRIU to keep it.
			if err := container.Destroy(); err != nil {
				logrus.Warn(err)
			}
		}
		return err
	},
}

func prepareImagePaths(cmd *cli.Command) (string, string, error) {
	imagePath := cmd.String("image-path")
	if imagePath == "" {
		imagePath = getDefaultImagePath()
	}

	if err := os.MkdirAll(imagePath, 0o600); err != nil {
		return "", "", err
	}

	parentPath := cmd.String("parent-path")
	if parentPath == "" {
		return imagePath, parentPath, nil
	}

	if filepath.IsAbs(parentPath) {
		return "", "", errors.New("--parent-path must be relative")
	}

	realParent := filepath.Join(imagePath, parentPath)
	fi, err := os.Stat(realParent)
	if err == nil && !fi.IsDir() {
		err = &os.PathError{Path: realParent, Err: unix.ENOTDIR}
	}

	if err != nil {
		return "", "", fmt.Errorf("invalid --parent-path: %w", err)
	}

	return imagePath, parentPath, nil
}

func criuOptions(cmd *cli.Command) (*libcontainer.CriuOpts, error) {
	imagePath, parentPath, err := prepareImagePaths(cmd)
	if err != nil {
		return nil, err
	}

	opts := &libcontainer.CriuOpts{
		ImagesDirectory:         imagePath,
		WorkDirectory:           cmd.String("work-path"),
		ParentImage:             parentPath,
		LeaveRunning:            cmd.Bool("leave-running"),
		TcpEstablished:          cmd.Bool("tcp-established"),
		TcpSkipInFlight:         cmd.Bool("tcp-skip-in-flight"),
		LinkRemap:               cmd.Bool("link-remap"),
		ExternalUnixConnections: cmd.Bool("ext-unix-sk"),
		ShellJob:                cmd.Bool("shell-job"),
		FileLocks:               cmd.Bool("file-locks"),
		PreDump:                 cmd.Bool("pre-dump"),
		AutoDedup:               cmd.Bool("auto-dedup"),
		LazyPages:               cmd.Bool("lazy-pages"),
		StatusFd:                cmd.Int("status-fd"),
		LsmProfile:              cmd.String("lsm-profile"),
		LsmMountContext:         cmd.String("lsm-mount-context"),
		ManageCgroupsMode:       cmd.String("manage-cgroups-mode"),
	}

	// CRIU options below may or may not be set.

	if psOpt := cmd.String("page-server"); psOpt != "" {
		address, port, err := net.SplitHostPort(psOpt)

		if err != nil || address == "" || port == "" {
			return nil, errors.New("Use --page-server ADDRESS:PORT to specify page server")
		}
		portInt, err := strconv.Atoi(port)
		if err != nil {
			return nil, errors.New("Invalid port number")
		}
		opts.PageServer = libcontainer.CriuPageServerInfo{
			Address: address,
			Port:    int32(portInt),
		}
	}

	// runc doesn't manage network devices and their configuration.
	nsmask := unix.CLONE_NEWNET

	if cmd.IsSet("empty-ns") {
		namespaceMapping := map[specs.LinuxNamespaceType]int{
			specs.NetworkNamespace: unix.CLONE_NEWNET,
		}

		for _, ns := range cmd.StringSlice("empty-ns") {
			f, exists := namespaceMapping[specs.LinuxNamespaceType(ns)]
			if !exists {
				return nil, fmt.Errorf("namespace %q is not supported", ns)
			}
			nsmask |= f
		}
	}

	opts.EmptyNs = uint32(nsmask)

	return opts, nil
}

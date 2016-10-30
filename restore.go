// +build linux

package main

import (
	"os"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var restoreCmd = &cobra.Command{
	Short: "restore a container from a previous checkpoint",
	Use: `restore [command options] <container-id>

Where "<container-id>" is the name for the instance of the container to be
restored.`,
	Long: `Restores the saved state of the container instance that was previously saved
using the runc checkpoint command.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		flags := cmd.Flags()

		if len(args) < 1 {
			return errEmptyID
		}
		id := args[0]
		imagePath, _ := flags.GetString("image-path")
		if imagePath == "" {
			imagePath = getDefaultImagePath()
		}
		if bundle, _ := flags.GetString("bundle"); bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				return err
			}
		}
		spec, err := loadSpec(specConfig)
		if err != nil {
			return err
		}
		config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
			CgroupName:       id,
			UseSystemdCgroup: func() bool { v, _ := flags.GetBool("systemd-cgroup"); return v }(),
			NoPivotRoot:      func() bool { v, _ := flags.GetBool("no-pivot"); return v }(),
			Spec:             spec,
		})
		if err != nil {
			return err
		}
		status, err := restoreContainer(id, flags, args, spec, config, imagePath)
		if err == nil {
			os.Exit(status)
		}
		return err
	},
}

func init() {
	flags := restoreCmd.Flags()

	flags.String("image-path", "", "path to criu image files for restoring")
	flags.String("work-path", "", "path for saving work files and logs")
	flags.Bool("tcp-established", false, "allow open tcp connections")
	flags.Bool("ext-unix-sk", false, "allow external unix sockets")
	flags.Bool("shell-job", false, "allow shell jobs")
	flags.Bool("file-locks", false, "handle file locks, for safety")
	flags.String("manage-cgroups-mode", "soft", "cgroups mode: 'soft', 'full' and 'strict'")
	flags.StringP("bundle", "b", "", "path to the root of the bundle directory")
	flags.BoolP("detach", "d", false, "detach from the container's process")
	flags.String("pid-file", "", "specify the file to write the process id to")
	flags.Bool("no-subreaper", false, "disable the use of the subreaper used to reap reparented processes")
	flags.Bool("no-pivot", false, "do not use pivot root to jail process inside rootfs.  This should be used whenever the rootfs is on top of a ramdisk")
	flags.StringSlice("empty-ns", []string{}, "create a namespace, but don't restore its properies")
}

func restoreContainer(id string, flags *pflag.FlagSet, args []string, spec *specs.Spec, config *configs.Config, imagePath string) (int, error) {
	var (
		rootuid = 0
		rootgid = 0
	)
	factory, err := loadFactory(flags)
	if err != nil {
		return -1, err
	}
	container, err := factory.Load(id)
	if err != nil {
		container, err = factory.Create(id, config)
		if err != nil {
			return -1, err
		}
	}
	options := criuOptions(flags)

	status, err := container.Status()
	if err != nil {
		logrus.Error(err)
	}
	if status == libcontainer.Running {
		fatalf("Container with id %s already running", id)
	}

	setManageCgroupsMode(flags, options)

	if err := setEmptyNsMask(flags, options); err != nil {
		return -1, err
	}

	// ensure that the container is always removed if we were the process
	// that created it.
	detach, _ := flags.GetBool("detach")
	if !detach {
		defer destroy(container)
	}
	process := &libcontainer.Process{}
	tty, err := setupIO(process, rootuid, rootgid, "", false, detach)
	if err != nil {
		return -1, err
	}
	defer tty.Close()
	noSubreaper, _ := flags.GetBool("no-subreaper")
	handler := newSignalHandler(tty, !noSubreaper)
	if err := container.Restore(process, options); err != nil {
		return -1, err
	}
	if err := tty.ClosePostStart(); err != nil {
		return -1, err
	}
	if pidFile, _ := flags.GetString("pid-file"); pidFile != "" {
		if err := createPidFile(pidFile, process); err != nil {
			process.Signal(syscall.SIGKILL)
			process.Wait()
			return -1, err
		}
	}
	if detach {
		return 0, nil
	}
	return handler.forward(process)
}

func criuOptions(flags *pflag.FlagSet) *libcontainer.CriuOpts {
	imagePath := getCheckpointImagePath(flags)
	if err := os.MkdirAll(imagePath, 0655); err != nil {
		fatal(err)
	}
	return &libcontainer.CriuOpts{
		ImagesDirectory:         imagePath,
		WorkDirectory:           func() string { v, _ := flags.GetString("work-path"); return v }(),
		LeaveRunning:            func() bool { v, _ := flags.GetBool("leave-running"); return v }(),
		TcpEstablished:          func() bool { v, _ := flags.GetBool("tcp-established"); return v }(),
		ExternalUnixConnections: func() bool { v, _ := flags.GetBool("ext-unix-sk"); return v }(),
		ShellJob:                func() bool { v, _ := flags.GetBool("shell-job"); return v }(),
		FileLocks:               func() bool { v, _ := flags.GetBool("file-locks"); return v }(),
	}
}

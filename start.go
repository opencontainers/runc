// +build linux

package main

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"syscall"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/specs"
)

const SD_LISTEN_FDS_START = 3

// default action is to start a container
var startCommand = cli.Command{
	Name:  "start",
	Usage: "create and run a container",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "bundle, b",
			Value: "",
			Usage: "path to the root of the bundle directory",
		},
		cli.StringFlag{
			Name:  "console",
			Value: "",
			Usage: "specify the pty slave path for use with the container",
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
	},
	Action: func(context *cli.Context) {
		bundle := context.String("bundle")
		if bundle != "" {
			if err := os.Chdir(bundle); err != nil {
				fatal(err)
			}
		}
		spec, rspec, err := loadSpec(specConfig, runtimeConfig)
		if err != nil {
			fatal(err)
		}

		notifySocket := os.Getenv("NOTIFY_SOCKET")
		if notifySocket != "" {
			setupSdNotify(spec, rspec, notifySocket)
		}

		var (
			listenFds = os.Getenv("LISTEN_FDS")
			listenPid = os.Getenv("LISTEN_PID")
		)
		if listenFds != "" && listenPid == strconv.Itoa(os.Getpid()) {
			setupSocketActivation(spec, listenFds)
		}

		if os.Geteuid() != 0 {
			logrus.Fatal("runc should be run as root")
		}
		status, err := startContainer(context, spec, rspec)
		if err != nil {
			logrus.Fatalf("Container start failed: %v", err)
		}
		// exit with the container's exit status so any external supervisor is
		// notified of the exit with the correct exit status.
		os.Exit(status)
	},
}

func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		runtime.GOMAXPROCS(1)
		runtime.LockOSThread()
		factory, _ := libcontainer.New("")
		if err := factory.StartInitialization(); err != nil {
			fatal(err)
		}
		panic("--this line should have never been executed, congratulations--")
	}
}

func startContainer(context *cli.Context, spec *specs.LinuxSpec, rspec *specs.LinuxRuntimeSpec) (int, error) {
	config, err := createLibcontainerConfig(context.GlobalString("id"), spec, rspec)
	if err != nil {
		return -1, err
	}
	if _, err := os.Stat(config.Rootfs); err != nil {
		if os.IsNotExist(err) {
			return -1, fmt.Errorf("Rootfs (%q) does not exist", config.Rootfs)
		}
		return -1, err
	}
	rootuid, err := config.HostUID()
	if err != nil {
		return -1, err
	}
	factory, err := loadFactory(context)
	if err != nil {
		return -1, err
	}
	container, err := factory.Create(context.GlobalString("id"), config)
	if err != nil {
		return -1, err
	}
	// ensure that the container is always removed if we were the process
	// that created it.
	detach := context.Bool("detach")
	if !detach {
		defer destroy(container)
	}
	process := newProcess(spec.Process)
	// Support on-demand socket activation by passing file descriptors into the container init process.
	if os.Getenv("LISTEN_FDS") != "" {
		listenFdsInt, err := strconv.Atoi(os.Getenv("LISTEN_FDS"))
		if err != nil {
			return -1, err
		}
		for i := SD_LISTEN_FDS_START; i < (listenFdsInt + SD_LISTEN_FDS_START); i++ {
			process.ExtraFiles = append(process.ExtraFiles, os.NewFile(uintptr(i), ""))
		}
	}
	tty, err := setupIO(process, rootuid, context.String("console"), spec.Process.Terminal, detach)
	if err != nil {
		return -1, err
	}
	if err := container.Start(process); err != nil {
		return -1, err
	}
	if pidFile := context.String("pid-file"); pidFile != "" {
		if err := createPidile(pidFile, process); err != nil {
			return -1, err
		}
	}
	if detach {
		return 0, nil
	}
	handler := newSignalHandler(tty)
	defer handler.Close()
	return handler.forward(process)
}

func dupStdio(process *libcontainer.Process, rootuid int) error {
	process.Stdin = os.Stdin
	process.Stdout = os.Stdout
	process.Stderr = os.Stderr
	for _, fd := range []uintptr{
		os.Stdin.Fd(),
		os.Stdout.Fd(),
		os.Stderr.Fd(),
	} {
		if err := syscall.Fchown(int(fd), rootuid, rootuid); err != nil {
			return err
		}
	}
	return nil
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func setupSdNotify(spec *specs.LinuxSpec, rspec *specs.LinuxRuntimeSpec, notifySocket string) {
	mountName := "sdNotify"
	spec.Mounts = append(spec.Mounts, specs.MountPoint{Name: mountName, Path: notifySocket})
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", notifySocket))
	rspec.Mounts[mountName] = specs.Mount{Type: "bind", Source: notifySocket, Options: []string{"bind"}}
}

// If systemd is supporting on-demand socket activation, this function will add support
// for on-demand socket activation for the containerized service.
func setupSocketActivation(spec *specs.LinuxSpec, listenFds string) {
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("LISTEN_FDS=%s", listenFds), "LISTEN_PID=1")
}

func destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

func setupIO(process *libcontainer.Process, rootuid int, console string, createTTY, detach bool) (*tty, error) {
	// detach and createTty will not work unless a console path is passed
	// so error out here before changing any terminal settings
	if createTTY && detach && console == "" {
		return nil, fmt.Errorf("cannot allocate tty if runc will detach")
	}
	if createTTY {
		return createTty(process, rootuid, console)
	}
	if detach {
		if err := dupStdio(process, rootuid); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return createStdioPipes(process, rootuid)
}

func createPidile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		logrus.WithField("pid", pid).Error("create pid file")
	} else {
		_, err = fmt.Fprintf(f, "%d", pid)
		f.Close()
		if err != nil {
			logrus.WithField("error", err).Error("write pid file")
		}
	}
	return nil
}

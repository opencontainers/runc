package main

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/moby/sys/userns"
	"github.com/opencontainers/runc/libcontainer/vtpm"
	vtpmhelper "github.com/opencontainers/runc/libcontainer/vtpm/vtpm-helper"
	"github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/libcontainer/system/kernelversion"
	"github.com/opencontainers/runc/libcontainer/utils"
)

var errEmptyID = errors.New("container id cannot be empty")

// getContainer returns the specified container instance by loading it from
// a state directory (root).
func getContainer(context *cli.Context) (*libcontainer.Container, error) {
	id := context.Args().First()
	if id == "" {
		return nil, errEmptyID
	}
	root := context.GlobalString("root")
	return libcontainer.Load(root, id)
}

func getDefaultImagePath() string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}

// newProcess converts [specs.Process] to [libcontainer.Process].
func newProcess(p *specs.Process) (*libcontainer.Process, error) {
	lp := &libcontainer.Process{
		Args:            p.Args,
		Env:             p.Env,
		UID:             int(p.User.UID),
		GID:             int(p.User.GID),
		Cwd:             p.Cwd,
		Label:           p.SelinuxLabel,
		NoNewPrivileges: &p.NoNewPrivileges,
		AppArmorProfile: p.ApparmorProfile,
		Scheduler:       p.Scheduler,
		IOPriority:      p.IOPriority,
	}

	if p.ConsoleSize != nil {
		lp.ConsoleWidth = uint16(p.ConsoleSize.Width)
		lp.ConsoleHeight = uint16(p.ConsoleSize.Height)
	}

	if p.Capabilities != nil {
		lp.Capabilities = &configs.Capabilities{}
		lp.Capabilities.Bounding = p.Capabilities.Bounding
		lp.Capabilities.Effective = p.Capabilities.Effective
		lp.Capabilities.Inheritable = p.Capabilities.Inheritable
		lp.Capabilities.Permitted = p.Capabilities.Permitted
		lp.Capabilities.Ambient = p.Capabilities.Ambient
	}
	if l := len(p.User.AdditionalGids); l > 0 {
		lp.AdditionalGroups = make([]int, l)
		for i, g := range p.User.AdditionalGids {
			lp.AdditionalGroups[i] = int(g)
		}
	}
	for _, rlimit := range p.Rlimits {
		rl, err := createLibContainerRlimit(rlimit)
		if err != nil {
			return nil, err
		}
		lp.Rlimits = append(lp.Rlimits, rl)
	}
	aff, err := configs.ConvertCPUAffinity(p.ExecCPUAffinity)
	if err != nil {
		return nil, err
	}
	lp.CPUAffinity = aff

	return lp, nil
}

// setupIO modifies the given process config according to the options.
func setupIO(process *libcontainer.Process, container *libcontainer.Container, createTTY, detach bool, sockpath string) (_ *tty, Err error) {
	if createTTY {
		process.Stdin = nil
		process.Stdout = nil
		process.Stderr = nil
		t := &tty{}
		if !detach {
			if err := t.initHostConsole(); err != nil {
				return nil, err
			}
			parent, child, err := utils.NewSockPair("console")
			if err != nil {
				return nil, err
			}
			process.ConsoleSocket = child
			t.postStart = append(t.postStart, parent, child)
			t.consoleC = make(chan error, 1)
			go func() {
				t.consoleC <- t.recvtty(parent)
			}()
		} else {
			// the caller of runc will handle receiving the console master
			conn, err := net.Dial("unix", sockpath)
			if err != nil {
				return nil, err
			}
			defer func() {
				if Err != nil {
					conn.Close()
				}
			}()
			t.postStart = append(t.postStart, conn)
			socket, err := conn.(*net.UnixConn).File()
			if err != nil {
				return nil, err
			}
			t.postStart = append(t.postStart, socket)
			process.ConsoleSocket = socket
		}
		return t, nil
	}
	// when runc will detach the caller provides the stdio to runc via runc's 0,1,2
	// and the container's process inherits runc's stdio.
	if detach {
		inheritStdio(process)
		return &tty{}, nil
	}

	config := container.Config()
	rootuid, err := config.HostRootUID()
	if err != nil {
		return nil, err
	}
	rootgid, err := config.HostRootGID()
	if err != nil {
		return nil, err
	}

	return setupProcessPipes(process, rootuid, rootgid)
}

// createPidFile creates a file containing the PID,
// doing so atomically (via create and rename).
func createPidFile(path string, process *libcontainer.Process) error {
	pid, err := process.Pid()
	if err != nil {
		return err
	}
	var (
		tmpDir  = filepath.Dir(path)
		tmpName = filepath.Join(tmpDir, "."+filepath.Base(path))
	)
	f, err := os.OpenFile(tmpName, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0o666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(strconv.Itoa(pid))
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func createContainer(context *cli.Context, id string, spec *specs.Spec, vtpms []*vtpm.VTPM) (*libcontainer.Container, error) {
	rootlessCg, err := shouldUseRootlessCgroupManager(context)
	if err != nil {
		return nil, err
	}
	config, err := specconv.CreateLibcontainerConfig(&specconv.CreateOpts{
		CgroupName:       id,
		UseSystemdCgroup: context.GlobalBool("systemd-cgroup"),
		NoPivotRoot:      context.Bool("no-pivot"),
		NoNewKeyring:     context.Bool("no-new-keyring"),
		Spec:             spec,
		RootlessEUID:     os.Geteuid() != 0,
		RootlessCgroups:  rootlessCg,
		VTPMs:            vtpms,
	})
	if err != nil {
		return nil, err
	}

	err = setHostDevsOwner(config)
	if err != nil {
		return nil, err
	}

	root := context.GlobalString("root")
	return libcontainer.Create(root, id, config)
}

type runner struct {
	init            bool
	enableSubreaper bool
	shouldDestroy   bool
	detach          bool
	listenFDs       []*os.File
	preserveFDs     int
	pidFile         string
	consoleSocket   string
	pidfdSocket     string
	container       *libcontainer.Container
	action          CtAct
	notifySocket    *notifySocket
	criuOpts        *libcontainer.CriuOpts
	subCgroupPaths  map[string]string
}

func (r *runner) run(config *specs.Process) (int, error) {
	var err error
	defer func() {
		if err != nil {
			r.destroy()
		}
	}()
	if err = r.checkTerminal(config); err != nil {
		return -1, err
	}
	process, err := newProcess(config)
	if err != nil {
		return -1, err
	}
	process.LogLevel = strconv.Itoa(int(logrus.GetLevel()))
	// Populate the fields that come from runner.
	process.Init = r.init
	process.SubCgroupPaths = r.subCgroupPaths
	if len(r.listenFDs) > 0 {
		process.Env = append(process.Env, "LISTEN_FDS="+strconv.Itoa(len(r.listenFDs)), "LISTEN_PID=1")
		process.ExtraFiles = append(process.ExtraFiles, r.listenFDs...)
	}
	baseFd := 3 + len(process.ExtraFiles)
	procSelfFd, closer := utils.ProcThreadSelf("fd/")
	defer closer()
	for i := baseFd; i < baseFd+r.preserveFDs; i++ {
		_, err = os.Stat(filepath.Join(procSelfFd, strconv.Itoa(i)))
		if err != nil {
			return -1, fmt.Errorf("unable to stat preserved-fd %d (of %d): %w", i-baseFd, r.preserveFDs, err)
		}
		process.ExtraFiles = append(process.ExtraFiles, os.NewFile(uintptr(i), "PreserveFD:"+strconv.Itoa(i)))
	}
	detach := r.detach || (r.action == CT_ACT_CREATE)
	// Setting up IO is a two stage process. We need to modify process to deal
	// with detaching containers, and then we get a tty after the container has
	// started.
	handlerCh := newSignalHandler(r.enableSubreaper, r.notifySocket)
	tty, err := setupIO(process, r.container, config.Terminal, detach, r.consoleSocket)
	if err != nil {
		return -1, err
	}
	defer tty.Close()

	if r.pidfdSocket != "" {
		connClose, err := setupPidfdSocket(process, r.pidfdSocket)
		if err != nil {
			return -1, err
		}
		defer connClose()
	}

	switch r.action {
	case CT_ACT_CREATE:
		err = r.container.Start(process)
	case CT_ACT_RESTORE:
		err = r.container.Restore(process, r.criuOpts)
	case CT_ACT_RUN:
		err = r.container.Run(process)
	default:
		panic("Unknown action")
	}
	if err != nil {
		return -1, err
	}
	if err = tty.waitConsole(); err != nil {
		r.terminate(process)
		return -1, err
	}
	tty.ClosePostStart()
	if r.pidFile != "" {
		if err = createPidFile(r.pidFile, process); err != nil {
			r.terminate(process)
			return -1, err
		}
	}
	handler := <-handlerCh
	status, err := handler.forward(process, tty, detach)
	if err != nil {
		r.terminate(process)
	}
	if detach {
		return 0, nil
	}
	if err == nil {
		r.destroy()
	}
	return status, err
}

func (r *runner) destroy() {
	if r.shouldDestroy {
		if err := r.container.Destroy(); err != nil {
			logrus.Warn(err)
		}
	}
}

func (r *runner) terminate(p *libcontainer.Process) {
	_ = p.Signal(unix.SIGKILL)
	_, _ = p.Wait()
}

func (r *runner) checkTerminal(config *specs.Process) error {
	detach := r.detach || (r.action == CT_ACT_CREATE)
	// Check command-line for sanity.
	if detach && config.Terminal && r.consoleSocket == "" {
		return errors.New("cannot allocate tty if runc will detach without setting console socket")
	}
	if (!detach || !config.Terminal) && r.consoleSocket != "" {
		return errors.New("cannot use console socket if runc will not detach or allocate tty")
	}
	return nil
}

func validateProcessSpec(spec *specs.Process) error {
	if spec == nil {
		return errors.New("process property must not be empty")
	}
	if spec.Cwd == "" {
		return errors.New("Cwd property must not be empty")
	}
	if !filepath.IsAbs(spec.Cwd) {
		return errors.New("Cwd must be an absolute path")
	}
	if len(spec.Args) == 0 {
		return errors.New("args must not be empty")
	}
	if spec.SelinuxLabel != "" && !selinux.GetEnabled() {
		return errors.New("selinux label is specified in config, but selinux is disabled or not supported")
	}
	return nil
}

type CtAct uint8

const (
	CT_ACT_CREATE CtAct = iota + 1
	CT_ACT_RUN
	CT_ACT_RESTORE
)

func startContainer(context *cli.Context, action CtAct, criuOpts *libcontainer.CriuOpts) (int, error) {
	if err := revisePidFile(context); err != nil {
		return -1, err
	}
	spec, err := setupSpec(context)
	if err != nil {
		return -1, err
	}

	id := context.Args().First()
	if id == "" {
		return -1, errEmptyID
	}

	notifySocket := newNotifySocket(context, os.Getenv("NOTIFY_SOCKET"), id)
	if notifySocket != nil {
		notifySocket.setupSpec(spec)
	}

	vtpms, err := createVTPMs(context.GlobalString("root"), id, spec)
	if err != nil {
		return -1, err
	}
	defer func() {
		if err != nil {
			destroyVTPMs(vtpms)
		}
	}()

	container, err := createContainer(context, id, spec, vtpms)
	if err != nil {
		return -1, err
	}

	if notifySocket != nil {
		if err := notifySocket.setupSocketDirectory(); err != nil {
			return -1, err
		}
		if action == CT_ACT_RUN {
			if err := notifySocket.bindSocket(); err != nil {
				return -1, err
			}
		}
	}

	// Support on-demand socket activation by passing file descriptors into the container init process.
	listenFDs := []*os.File{}
	if os.Getenv("LISTEN_FDS") != "" {
		listenFDs = activation.Files(false)
	}

	r := &runner{
		enableSubreaper: !context.Bool("no-subreaper"),
		shouldDestroy:   !context.Bool("keep"),
		container:       container,
		listenFDs:       listenFDs,
		notifySocket:    notifySocket,
		consoleSocket:   context.String("console-socket"),
		pidfdSocket:     context.String("pidfd-socket"),
		detach:          context.Bool("detach"),
		pidFile:         context.String("pid-file"),
		preserveFDs:     context.Int("preserve-fds"),
		action:          action,
		criuOpts:        criuOpts,
		init:            true,
	}
	return r.run(spec.Process)
}

func setupPidfdSocket(process *libcontainer.Process, sockpath string) (_clean func(), _ error) {
	linux530 := kernelversion.KernelVersion{Kernel: 5, Major: 3}
	ok, err := kernelversion.GreaterEqualThan(linux530)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("--pidfd-socket requires >= v5.3 kernel")
	}

	conn, err := net.Dial("unix", sockpath)
	if err != nil {
		return nil, fmt.Errorf("failed to dail %s: %w", sockpath, err)
	}

	socket, err := conn.(*net.UnixConn).File()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to dup socket: %w", err)
	}

	process.PidfdSocket = socket
	return func() {
		conn.Close()
	}, nil
}

func maybeLogCgroupWarning(op string, err error) {
	if errors.Is(err, fs.ErrPermission) {
		logrus.Warn("runc " + op + " failure might be caused by lack of full access to cgroups")
	}
}

// addVTPMDevice adds a device and cgroup entry to the spec
func addVTPMDevice(spec *specs.Spec, devpath string, major, minor uint32) {
	var filemode os.FileMode = 0600

	device := specs.LinuxDevice{
		Path:     devpath,
		Type:     "c",
		Major:    int64(major),
		Minor:    int64(minor),
		FileMode: &filemode,
	}
	spec.Linux.Devices = append(spec.Linux.Devices, device)

	major_p := new(int64)
	*major_p = int64(major)
	minor_p := new(int64)
	*minor_p = int64(minor)

	ld := &specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   "c",
		Major:  major_p,
		Minor:  minor_p,
		Access: "rwm",
	}
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, *ld)
}

func ContainsUserNamespace(namespaces []specs.LinuxNamespace, userNs specs.LinuxNamespaceType) bool {
	for _, ns := range namespaces {
		if ns.Type == userNs {
			return true
		}
	}
	return false
}

func createVTPMs(root, containerID string, spec *specs.Spec) ([]*vtpm.VTPM, error) {
	var vtpms []*vtpm.VTPM
	type vtpmLinuxDevice struct {
		devPath string
		major   uint32
		minor   uint32
	}
	var devices []vtpmLinuxDevice

	r := spec.Linux.Resources
	if r == nil {
		return vtpms, nil
	}

	err := vtpmhelper.CheckVTPMParams(r.VTPMs)
	if err != nil {
		if vtpmhelper.CanIgnoreVTPMErrors() {
			logrus.Errorf("createVTPMs has an error: %s", err)
			return nil, nil
		}
		return nil, err
	}

	for ind, vtpmSpec := range r.VTPMs {
		vtpm, device, err := func(vtpmSpec specs.LinuxVTPM) (*vtpm.VTPM, vtpmLinuxDevice, error) {
			var major uint32
			var minor uint32
			var fileInfo os.FileInfo
			var err error
			var vtpm *vtpm.VTPM
			containerVTPMName := vtpmSpec.VTPMName

			// Several containers can have vtpms on the same devpath that's why we need to create unique host path.
			vtpmSpec.VTPMName = vtpmhelper.GenerateDeviceHostPathName(root, containerID, containerVTPMName)
			hostdev := "/dev/tpm" + vtpmSpec.VTPMName

			if fileInfo, err = os.Lstat(hostdev); err != nil && os.IsNotExist(err) {
				vtpm, err = vtpmhelper.CreateVTPM(spec, &vtpmSpec)
				if err != nil {
					return nil, vtpmLinuxDevice{}, fmt.Errorf("createVTPMs has an error while create %d VTPM device: %w", ind, err)
				}
			} else if err != nil {
				return nil, vtpmLinuxDevice{}, fmt.Errorf("createVTPMs has an error while checking dev path %s: %s", hostdev, err)
			}

			fileInfo, err = os.Lstat(hostdev)
			if err != nil {
				return vtpm, vtpmLinuxDevice{}, fmt.Errorf("createVTPMs has an error while checking dev path %s: %s", hostdev, err)
			}

			stat_t, ok := fileInfo.Sys().(*syscall.Stat_t)
			if !ok {
				return vtpm, vtpmLinuxDevice{}, fmt.Errorf("createVTPMs has an error while checking info type for device %s: %w", hostdev, err)
			}

			devNumber := stat_t.Rdev
			major = unix.Major(devNumber)
			minor = unix.Minor(devNumber)

			devpath := "/dev/tpm" + containerVTPMName
			// We switch to the created device name because in runc's init we will use bind command instead mknod
			if userns.RunningInUserNS() || spec.Linux != nil && ContainsUserNamespace(spec.Linux.Namespaces, specs.UserNamespace) {
				devpath = hostdev
			}

			device := vtpmLinuxDevice{
				devPath: devpath,
				major:   major,
				minor:   minor,
			}
			return vtpm, device, nil
		}(vtpmSpec)

		if vtpm != nil {
			vtpms = append(vtpms, vtpm)
		}
		if err != nil {
			if vtpmhelper.CanIgnoreVTPMErrors() {
				logrus.Error(err)
				continue
			}
			destroyVTPMs(vtpms)
			return nil, err
		}
		devices = append(devices, device)
	}
	for _, device := range devices {
		addVTPMDevice(spec, device.devPath, device.major, device.minor)
	}

	return vtpms, nil
}

func destroyVTPMs(vtpms []*vtpm.VTPM) {
	if len(vtpms) > 0 {
		vtpmhelper.DestroyVTPMs(vtpms)
	}
}

func setVTPMHostDevsOwner(config *configs.Config) error {
	rootUID, err := config.HostRootUID()
	if err != nil {
		return err
	}
	rootGID, err := config.HostRootGID()
	if err != nil {
		return err
	}
	return vtpmhelper.SetVTPMHostDevsOwner(config, rootUID, rootGID)
}

func setHostDevsOwner(config *configs.Config) error {
	return setVTPMHostDevsOwner(config)
}

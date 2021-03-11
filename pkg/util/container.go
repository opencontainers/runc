// +build linux

package util

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/specconv"
	"github.com/opencontainers/runc/pkg/constant"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/selinux/go-selinux"
)

var errEmptyID = errors.New("container id cannot be empty")

// LoadFactory returns the configured factory instance for execing containers.
func LoadFactory(context *cli.Context) (libcontainer.Factory, error) {
	root := context.GlobalString("root")
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	// We default to cgroupfs, and can only use systemd if the system is a
	// systemd box.
	cgroupManager := libcontainer.Cgroupfs
	rootlessCg, err := ShouldUseRootlessCgroupManager(context)
	if err != nil {
		return nil, err
	}
	if rootlessCg {
		cgroupManager = libcontainer.RootlessCgroupfs
	}
	if context.GlobalBool("systemd-cgroup") {
		if !systemd.IsRunningSystemd() {
			return nil, errors.New("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
		cgroupManager = libcontainer.SystemdCgroups
		if rootlessCg {
			cgroupManager = libcontainer.RootlessSystemdCgroups
		}
	}

	intelRdtManager := libcontainer.IntelRdtFs

	// We resolve the paths for {newuidmap,newgidmap} from the context of runc,
	// to avoid doing a path lookup in the nsexec context. TODO: The binary
	// names are not currently configurable.
	newuidmap, err := exec.LookPath("newuidmap")
	if err != nil {
		newuidmap = ""
	}
	newgidmap, err := exec.LookPath("newgidmap")
	if err != nil {
		newgidmap = ""
	}

	return libcontainer.New(abs, cgroupManager, intelRdtManager,
		libcontainer.CriuPath(context.GlobalString("criu")),
		libcontainer.NewuidmapPath(newuidmap),
		libcontainer.NewgidmapPath(newgidmap))
}

// GetContainer returns the specified container instance by loading it from state
// with the default factory.
func GetContainer(context *cli.Context) (libcontainer.Container, error) {
	id := context.Args().First()
	if id == "" {
		return nil, errEmptyID
	}
	factory, err := LoadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Load(id)
}

func GetDefaultImagePath(context *cli.Context) string {
	cwd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return filepath.Join(cwd, "checkpoint")
}

func Destroy(container libcontainer.Container) {
	if err := container.Destroy(); err != nil {
		logrus.Error(err)
	}
}

func createContainer(context *cli.Context, id string, spec *specs.Spec) (libcontainer.Container, error) {
	rootlessCg, err := ShouldUseRootlessCgroupManager(context)
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
	})
	if err != nil {
		return nil, err
	}

	factory, err := LoadFactory(context)
	if err != nil {
		return nil, err
	}
	return factory.Create(id, config)
}

func StartContainer(context *cli.Context, spec *specs.Spec, action CtAct, criuOpts *libcontainer.CriuOpts) (int, error) {
	id := context.Args().First()
	if id == "" {
		return -1, errEmptyID
	}

	notifySocket := newNotifySocket(context, os.Getenv("NOTIFY_SOCKET"), id)
	if notifySocket != nil {
		if err := notifySocket.setupSpec(context, spec); err != nil {
			return -1, err
		}
	}

	container, err := createContainer(context, id, spec)
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

	logLevel := "info"
	if context.GlobalBool("debug") {
		logLevel = "debug"
	}

	r := &Runner{
		EnableSubreaper: !context.Bool("no-subreaper"),
		ShouldDestroy:   true,
		Container:       container,
		ListenFDs:       listenFDs,
		NotifySocket:    notifySocket,
		ConsoleSocket:   context.String("console-socket"),
		Detach:          context.Bool("Detach"),
		PidFile:         context.String("pid-file"),
		PreserveFDs:     context.Int("preserve-fds"),
		Action:          action,
		CriuOpts:        criuOpts,
		Init:            true,
		LogLevel:        logLevel,
	}
	return r.Run(spec.Process)
}

// SetupSpec performs initial setup based on the cli.Context for the Container
func SetupSpec(context *cli.Context) (*specs.Spec, error) {
	bundle := context.String("bundle")
	if bundle != "" {
		if err := os.Chdir(bundle); err != nil {
			return nil, err
		}
	}
	spec, err := LoadSpec(constant.SpecConfig)
	if err != nil {
		return nil, err
	}
	return spec, nil
}

// LoadSpec loads the specification from the provided path.
func LoadSpec(cPath string) (spec *specs.Spec, err error) {
	cf, err := os.Open(cPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("JSON specification file %s not found", cPath)
		}
		return nil, err
	}
	defer cf.Close()

	if err = json.NewDecoder(cf).Decode(&spec); err != nil {
		return nil, err
	}
	return spec, ValidateProcessSpec(spec.Process)
}

func ValidateProcessSpec(spec *specs.Process) error {
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

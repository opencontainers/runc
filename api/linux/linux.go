package linux

import (
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/opencontainers/runc/api"
	"github.com/opencontainers/runc/api/command"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/cgroups/systemd"
	"github.com/opencontainers/runc/libcontainer/intelrdt"
)

func New(config command.GlobalConfig) (api.ContainerOperations, error) {
	abs, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	return &Libcontainer{
		root:          abs,
		criuPath:      config.CriuPath,
		systemdCgroup: config.SystemdCgroup,
	}, nil
}

type Libcontainer struct {
	root          string
	criuPath      string
	systemdCgroup bool
}

// loadFactory returns the configured factory instance for execing containers.
func (l *Libcontainer) loadFactory() (libcontainer.Factory, error) {
	// We default to cgroupfs, and can only use systemd if the system is a
	// systemd box.
	cgroupManager := libcontainer.Cgroupfs
	if l.systemdCgroup {
		if systemd.UseSystemd() {
			cgroupManager = libcontainer.SystemdCgroups
		} else {
			return nil, fmt.Errorf("systemd cgroup flag passed, but systemd support for managing cgroups is not available")
		}
	}

	intelRdtManager := libcontainer.IntelRdtFs
	if !intelrdt.IsEnabled() {
		intelRdtManager = nil
	}

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

	return libcontainer.New(l.root, cgroupManager, intelRdtManager,
		libcontainer.CriuPath(l.criuPath),
		libcontainer.NewuidmapPath(newuidmap),
		libcontainer.NewgidmapPath(newgidmap))
}

// getContainer returns the specified container instance by loading it from state
// with the default factory.
func (l *Libcontainer) getContainer(id string) (libcontainer.Container, error) {
	factory, err := l.loadFactory()
	if err != nil {
		return nil, err
	}
	return factory.Load(id)
}

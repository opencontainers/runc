// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/security/capabilities"
	"github.com/docker/libcontainer/system"
	"github.com/docker/libcontainer/user"
	"github.com/docker/libcontainer/utils"
)

type initType string

const (
	initSetns       initType = "setns"
	initStandard    initType = "standard"
	initUserns      initType = "userns"
	initUsernsSetup initType = "userns_setup"
)

// Process is used for transferring parameters from Exec() to Init()
type initConfig struct {
	Args   []string        `json:"args,omitempty"`
	Env    []string        `json:"env,omitempty"`
	Config *configs.Config `json:"config,omitempty"`
}

type initer interface {
	Init() error
}

func newContainerInit(t initType, pipe *os.File) (initer, error) {
	var config *initConfig
	if err := json.NewDecoder(pipe).Decode(&config); err != nil {
		return nil, err
	}
	if err := populateProcessEnvironment(config.Env); err != nil {
		return nil, err
	}
	switch t {
	case initSetns:
		return &linuxSetnsInit{
			args:   config.Args,
			env:    config.Env,
			config: config.Config,
		}, nil
	case initUserns:
		return &linuxUsernsInit{
			args:   config.Args,
			env:    config.Env,
			config: config.Config,
		}, nil
	case initUsernsSetup:
		return &linuxUsernsSideCar{
			config: config.Config,
		}, nil
	case initStandard:
		return &linuxStandardInit{
			config: config,
			env:    config.Env,
		}, nil
	}
	return nil, fmt.Errorf("unknown init type %q", t)
}

// populateProcessEnvironment loads the provided environment variables into the
// current processes's environment.
func populateProcessEnvironment(env []string) error {
	for _, pair := range env {
		p := strings.SplitN(pair, "=", 2)
		if len(p) < 2 {
			return fmt.Errorf("invalid environment '%v'", pair)
		}
		if err := os.Setenv(p[0], p[1]); err != nil {
			return err
		}
	}
	return nil
}

// finalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaky file descriptors
// before execing the command inside the namespace
func finalizeNamespace(config *configs.Config) error {
	// Ensure that all non-standard fds we may have accidentally
	// inherited are marked close-on-exec so they stay out of the
	// container
	if err := utils.CloseExecFrom(3); err != nil {
		return err
	}
	// drop capabilities in bounding set before changing user
	if err := capabilities.DropBoundingSet(config.Capabilities); err != nil {
		return err
	}
	// preserve existing capabilities while we change users
	if err := system.SetKeepCaps(); err != nil {
		return err
	}
	if err := setupUser(config); err != nil {
		return err
	}
	if err := system.ClearKeepCaps(); err != nil {
		return err
	}
	// drop all other capabilities
	if err := capabilities.DropCapabilities(config.Capabilities); err != nil {
		return err
	}
	if config.WorkingDir != "" {
		if err := syscall.Chdir(config.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", config.WorkingDir, err)
		}
	}
	return nil
}

// joinExistingNamespaces gets all the namespace paths specified for the container and
// does a setns on the namespace fd so that the current process joins the namespace.
func joinExistingNamespaces(namespaces []configs.Namespace) error {
	for _, ns := range namespaces {
		if ns.Path != "" {
			f, err := os.OpenFile(ns.Path, os.O_RDONLY, 0)
			if err != nil {
				return err
			}
			err = system.Setns(f.Fd(), uintptr(ns.Syscall()))
			f.Close()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// setupUser changes the groups, gid, and uid for the user inside the container
func setupUser(config *configs.Config) error {
	// Set up defaults.
	defaultExecUser := user.ExecUser{
		Uid:  syscall.Getuid(),
		Gid:  syscall.Getgid(),
		Home: "/",
	}
	passwdPath, err := user.GetPasswdPath()
	if err != nil {
		return err
	}
	groupPath, err := user.GetGroupPath()
	if err != nil {
		return err
	}
	execUser, err := user.GetExecUserPath(config.User, &defaultExecUser, passwdPath, groupPath)
	if err != nil {
		return fmt.Errorf("get supplementary groups %s", err)
	}
	suppGroups := append(execUser.Sgids, config.AdditionalGroups...)
	if err := syscall.Setgroups(suppGroups); err != nil {
		return fmt.Errorf("setgroups %s", err)
	}
	if err := system.Setgid(execUser.Gid); err != nil {
		return fmt.Errorf("setgid %s", err)
	}
	if err := system.Setuid(execUser.Uid); err != nil {
		return fmt.Errorf("setuid %s", err)
	}
	// if we didn't get HOME already, set it based on the user's HOME
	if envHome := os.Getenv("HOME"); envHome == "" {
		if err := os.Setenv("HOME", execUser.Home); err != nil {
			return fmt.Errorf("set HOME %s", err)
		}
	}
	return nil
}

// setupVethNetwork uses the Network config if it is not nil to initialize
// the new veth interface inside the container for use by changing the name to eth0
// setting the MTU and IP address along with the default gateway
func setupNetwork(config *configs.Config) error {
	for _, config := range config.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		err1 := strategy.Initialize(config)
		if err1 != nil {
			return err1
		}
	}
	return nil
}

func setupRoute(config *configs.Config) error {
	for _, config := range config.Routes {
		if err := netlink.AddRoute(config.Destination, config.Source, config.Gateway, config.InterfaceName); err != nil {
			return err
		}
	}
	return nil
}

func setupRlimits(config *configs.Config) error {
	for _, rlimit := range config.Rlimits {
		l := &syscall.Rlimit{Max: rlimit.Hard, Cur: rlimit.Soft}
		if err := syscall.Setrlimit(rlimit.Type, l); err != nil {
			return fmt.Errorf("error setting rlimit type %v: %v", rlimit.Type, err)
		}
	}
	return nil
}

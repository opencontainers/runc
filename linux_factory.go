// +build linux

package libcontainer

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/golang/glog"

	"github.com/docker/libcontainer/apparmor"
	cgroups "github.com/docker/libcontainer/cgroups/manager"
	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/console"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/mount"
	"github.com/docker/libcontainer/netlink"
	"github.com/docker/libcontainer/network"
	"github.com/docker/libcontainer/security/capabilities"
	"github.com/docker/libcontainer/security/restrict"
	"github.com/docker/libcontainer/system"
	"github.com/docker/libcontainer/user"
	"github.com/docker/libcontainer/utils"
)

const (
	configFilename = "config.json"
	stateFilename  = "state.json"
)

var (
	idRegex  = regexp.MustCompile(`^[\w_]+$`)
	maxIdLen = 1024
)

// Process is used for transferring parameters from Exec() to Init()
type processArgs struct {
	Args         []string              `json:"args,omitempty"`
	Config       *configs.Config       `json:"config,omitempty"`
	NetworkState *configs.NetworkState `json:"network_state,omitempty"`
}

// New returns a linux based container factory based in the root directory.
func New(root string, initArgs []string) (Factory, error) {
	if root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			return nil, newGenericError(err, SystemError)
		}
	}
	return &linuxFactory{
		root:     root,
		initArgs: initArgs,
	}, nil
}

// linuxFactory implements the default factory interface for linux based systems.
type linuxFactory struct {
	// root is the root directory
	root     string
	initArgs []string
}

func (l *linuxFactory) Create(id string, config *configs.Config) (Container, error) {
	if l.root == "" {
		return nil, newGenericError(fmt.Errorf("invalid root"), ConfigInvalid)
	}
	if err := l.validateID(id); err != nil {
		return nil, err
	}
	containerRoot := filepath.Join(l.root, id)
	if _, err := os.Stat(containerRoot); err == nil {
		return nil, newGenericError(fmt.Errorf("Container with id exists: %v", id), IdInUse)
	} else if !os.IsNotExist(err) {
		return nil, newGenericError(err, SystemError)
	}
	data, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return nil, newGenericError(err, SystemError)
	}
	if err := os.MkdirAll(containerRoot, 0700); err != nil {
		return nil, newGenericError(err, SystemError)
	}
	f, err := os.Create(filepath.Join(containerRoot, configFilename))
	if err != nil {
		os.RemoveAll(containerRoot)
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.RemoveAll(containerRoot)
		return nil, newGenericError(err, SystemError)
	}
	cgroupManager := cgroups.NewCgroupManager(config.Cgroups)
	return &linuxContainer{
		id:            id,
		root:          containerRoot,
		config:        config,
		initArgs:      l.initArgs,
		state:         &configs.State{},
		cgroupManager: cgroupManager,
	}, nil
}

func (l *linuxFactory) Load(id string) (Container, error) {
	if l.root == "" {
		return nil, newGenericError(fmt.Errorf("invalid root"), ConfigInvalid)
	}
	containerRoot := filepath.Join(l.root, id)
	glog.Infof("loading container config from %s", containerRoot)
	config, err := l.loadContainerConfig(containerRoot)
	if err != nil {
		return nil, err
	}
	glog.Infof("loading container state from %s", containerRoot)
	state, err := l.loadContainerState(containerRoot)
	if err != nil {
		return nil, err
	}
	cgroupManager := cgroups.LoadCgroupManager(config.Cgroups, state.CgroupPaths)
	glog.Infof("using %s as cgroup manager", cgroupManager)
	return &linuxContainer{
		id:            id,
		root:          containerRoot,
		config:        config,
		state:         state,
		cgroupManager: cgroupManager,
		initArgs:      l.initArgs,
	}, nil
}

// StartInitialization loads a container by opening the pipe fd from the parent to read the configuration and state
// This is a low level implementation detail of the reexec and should not be consumed externally
func (l *linuxFactory) StartInitialization(pipefd uintptr) (err error) {
	pipe := os.NewFile(uintptr(pipefd), "pipe")
	setupUserns := os.Getenv("_LIBCONTAINER_USERNS") != ""
	pid := os.Getenv("_LIBCONTAINER_INITPID")
	if pid != "" && !setupUserns {
		return initIn(pipe)
	}
	defer func() {
		// if we have an error during the initialization of the container's init then send it back to the
		// parent process in the form of an initError.
		if err != nil {
			// ensure that any data sent from the parent is consumed so it doesn't
			// receive ECONNRESET when the child writes to the pipe.
			ioutil.ReadAll(pipe)
			if err := json.NewEncoder(pipe).Encode(initError{
				Message: err.Error(),
			}); err != nil {
				panic(err)
			}
		}
		// ensure that this pipe is always closed
		pipe.Close()
	}()
	uncleanRootfs, err := os.Getwd()
	if err != nil {
		return err
	}
	var process *processArgs
	// We always read this as it is a way to sync with the parent as well
	if err := json.NewDecoder(pipe).Decode(&process); err != nil {
		return err
	}
	if setupUserns {
		err = setupContainer(process)
		if err == nil {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
	if process.Config.Namespaces.Contains(configs.NEWUSER) {
		return l.initUserNs(uncleanRootfs, process)
	}
	return l.initDefault(uncleanRootfs, process)
}

func (l *linuxFactory) loadContainerConfig(root string) (*configs.Config, error) {
	f, err := os.Open(filepath.Join(root, configFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newGenericError(err, ContainerNotExists)
		}
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()
	var config *configs.Config
	if err := json.NewDecoder(f).Decode(&config); err != nil {
		return nil, newGenericError(err, ConfigInvalid)
	}
	return config, nil
}

func (l *linuxFactory) loadContainerState(root string) (*configs.State, error) {
	f, err := os.Open(filepath.Join(root, stateFilename))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newGenericError(err, ContainerNotExists)
		}
		return nil, newGenericError(err, SystemError)
	}
	defer f.Close()
	var state *configs.State
	if err := json.NewDecoder(f).Decode(&state); err != nil {
		return nil, newGenericError(err, SystemError)
	}
	return state, nil
}

func (l *linuxFactory) validateID(id string) error {
	if !idRegex.MatchString(id) {
		return newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}
	if len(id) > maxIdLen {
		return newGenericError(fmt.Errorf("Invalid id format: %v", id), InvalidIdFormat)
	}
	return nil
}

func (l *linuxFactory) initDefault(uncleanRootfs string, process *processArgs) (err error) {
	config := process.Config
	networkState := process.NetworkState

	// TODO: move to validation
	/*
		rootfs, err := utils.ResolveRootfs(uncleanRootfs)
		if err != nil {
			return err
		}
	*/

	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := loadContainerEnvironment(config); err != nil {
		return err
	}
	// join any namespaces via a path to the namespace fd if provided
	if err := joinExistingNamespaces(config.Namespaces); err != nil {
		return err
	}
	if config.Console != "" {
		if err := console.OpenAndDup(config.Console); err != nil {
			return err
		}
	}
	if _, err := syscall.Setsid(); err != nil {
		return fmt.Errorf("setsid %s", err)
	}
	if config.Console != "" {
		if err := system.Setctty(); err != nil {
			return fmt.Errorf("setctty %s", err)
		}
	}

	cloneFlags := config.Namespaces.CloneFlags()
	if (cloneFlags & syscall.CLONE_NEWNET) == 0 {
		if len(config.Networks) != 0 || len(config.Routes) != 0 {
			return fmt.Errorf("unable to apply network parameters without network namespace")
		}
	} else {
		if err := setupNetwork(config, networkState); err != nil {
			return fmt.Errorf("setup networking %s", err)
		}
		if err := setupRoute(config); err != nil {
			return fmt.Errorf("setup route %s", err)
		}
	}
	if err := setupRlimits(config); err != nil {
		return fmt.Errorf("setup rlimits %s", err)
	}
	label.Init()
	// InitializeMountNamespace() can be executed only for a new mount namespace
	if (cloneFlags & syscall.CLONE_NEWNS) != 0 {
		if err := mount.InitializeMountNamespace(config); err != nil {
			return err
		}
	}
	if config.Hostname != "" {
		// TODO: (crosbymichael) move this to pre spawn validation
		if (cloneFlags & syscall.CLONE_NEWUTS) == 0 {
			return fmt.Errorf("unable to set the hostname without UTS namespace")
		}
		if err := syscall.Sethostname([]byte(config.Hostname)); err != nil {
			return fmt.Errorf("unable to sethostname %q: %s", config.Hostname, err)
		}
	}
	if err := apparmor.ApplyProfile(config.AppArmorProfile); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", config.AppArmorProfile, err)
	}
	if err := label.SetProcessLabel(config.ProcessLabel); err != nil {
		return fmt.Errorf("set process label %s", err)
	}
	// TODO: (crosbymichael) make this configurable at the Config level
	if config.RestrictSys {
		if (cloneFlags & syscall.CLONE_NEWNS) == 0 {
			return fmt.Errorf("unable to restrict access to kernel files without mount namespace")
		}
		if err := restrict.Restrict("proc/sys", "proc/sysrq-trigger", "proc/irq", "proc/bus"); err != nil {
			return err
		}
	}
	pdeathSignal, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("get parent death signal %s", err)
	}
	if err := finalizeNamespace(config); err != nil {
		return fmt.Errorf("finalize namespace %s", err)
	}
	// finalizeNamespace can change user/group which clears the parent death
	// signal, so we restore it here.
	if err := restoreParentDeathSignal(pdeathSignal); err != nil {
		return fmt.Errorf("restore parent death signal %s", err)
	}
	return system.Execv(process.Args[0], process.Args[0:], config.Env)
}

func (l *linuxFactory) initUserNs(uncleanRootfs string, process *processArgs) (err error) {
	config := process.Config
	// clear the current processes env and replace it with the environment
	// defined on the config
	if err := loadContainerEnvironment(config); err != nil {
		return err
	}
	// join any namespaces via a path to the namespace fd if provided
	if err := joinExistingNamespaces(config.Namespaces); err != nil {
		return err
	}
	if config.Console != "" {
		if err := console.OpenAndDup("/dev/console"); err != nil {
			return err
		}
	}
	if _, err := syscall.Setsid(); err != nil {
		return fmt.Errorf("setsid %s", err)
	}
	if config.Console != "" {
		if err := system.Setctty(); err != nil {
			return fmt.Errorf("setctty %s", err)
		}
	}
	if config.WorkingDir == "" {
		config.WorkingDir = "/"
	}
	if err := setupRlimits(config); err != nil {
		return fmt.Errorf("setup rlimits %s", err)
	}
	cloneFlags := config.Namespaces.CloneFlags()
	if config.Hostname != "" {
		// TODO: move validation
		if (cloneFlags & syscall.CLONE_NEWUTS) == 0 {
			return fmt.Errorf("unable to set the hostname without UTS namespace")
		}
		if err := syscall.Sethostname([]byte(config.Hostname)); err != nil {
			return fmt.Errorf("unable to sethostname %q: %s", config.Hostname, err)
		}
	}
	if err := apparmor.ApplyProfile(config.AppArmorProfile); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", config.AppArmorProfile, err)
	}
	if err := label.SetProcessLabel(config.ProcessLabel); err != nil {
		return fmt.Errorf("set process label %s", err)
	}
	if config.RestrictSys {
		if (cloneFlags & syscall.CLONE_NEWNS) == 0 {
			return fmt.Errorf("unable to restrict access to kernel files without mount namespace")
		}
		if err := restrict.Restrict("proc/sys", "proc/sysrq-trigger", "proc/irq", "proc/bus"); err != nil {
			return err
		}
	}
	pdeathSignal, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("get parent death signal %s", err)
	}
	if err := finalizeNamespace(config); err != nil {
		return fmt.Errorf("finalize namespace %s", err)
	}
	// finalizeNamespace can change user/group which clears the parent death
	// signal, so we restore it here.
	if err := restoreParentDeathSignal(pdeathSignal); err != nil {
		return fmt.Errorf("restore parent death signal %s", err)
	}
	return system.Execv(process.Args[0], process.Args[0:], config.Env)
}

// restoreParentDeathSignal sets the parent death signal to old.
func restoreParentDeathSignal(old int) error {
	if old == 0 {
		return nil
	}
	current, err := system.GetParentDeathSignal()
	if err != nil {
		return fmt.Errorf("get parent death signal %s", err)
	}
	if old == current {
		return nil
	}
	if err := system.ParentDeathSignal(uintptr(old)); err != nil {
		return fmt.Errorf("set parent death signal %s", err)
	}
	// Signal self if parent is already dead. Does nothing if running in a new
	// PID namespace, as Getppid will always return 0.
	if syscall.Getppid() == 1 {
		return syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
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
func setupNetwork(config *configs.Config, networkState *configs.NetworkState) error {
	for _, config := range config.Networks {
		strategy, err := network.GetStrategy(config.Type)
		if err != nil {
			return err
		}
		err1 := strategy.Initialize(config, networkState)
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

// finalizeNamespace drops the caps, sets the correct user
// and working dir, and closes any leaky file descriptors
// before execing the command inside the namespace
func finalizeNamespace(config *configs.Config) error {
	// Ensure that all non-standard fds we may have accidentally
	// inherited are marked close-on-exec so they stay out of the
	// container
	if err := utils.CloseExecFrom(3); err != nil {
		return fmt.Errorf("close open file descriptors %s", err)
	}
	// drop capabilities in bounding set before changing user
	if err := capabilities.DropBoundingSet(config.Capabilities); err != nil {
		return fmt.Errorf("drop bounding set %s", err)
	}
	// preserve existing capabilities while we change users
	if err := system.SetKeepCaps(); err != nil {
		return fmt.Errorf("set keep caps %s", err)
	}
	if err := setupUser(config); err != nil {
		return fmt.Errorf("setup user %s", err)
	}
	if err := system.ClearKeepCaps(); err != nil {
		return fmt.Errorf("clear keep caps %s", err)
	}
	// drop all other capabilities
	if err := capabilities.DropCapabilities(config.Capabilities); err != nil {
		return fmt.Errorf("drop capabilities %s", err)
	}
	if config.WorkingDir != "" {
		if err := syscall.Chdir(config.WorkingDir); err != nil {
			return fmt.Errorf("chdir to %s %s", config.WorkingDir, err)
		}
	}
	return nil
}

func loadContainerEnvironment(config *configs.Config) error {
	os.Clearenv()
	for _, pair := range config.Env {
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

// setupContainer is run to setup mounts and networking related operations
// for a user namespace enabled process as a user namespace root doesn't
// have permissions to perform these operations.
// The setup process joins all the namespaces of user namespace enabled init
// except the user namespace, so it run as root in the root user namespace
// to perform these operations.
func setupContainer(process *processArgs) error {
	container := process.Config
	networkState := process.NetworkState

	// TODO : move to validation
	/*
		rootfs, err := utils.ResolveRootfs(container.Rootfs)
		if err != nil {
			return err
		}
	*/

	// clear the current processes env and replace it with the environment
	// defined on the container
	if err := loadContainerEnvironment(container); err != nil {
		return err
	}

	cloneFlags := container.Namespaces.CloneFlags()
	if (cloneFlags & syscall.CLONE_NEWNET) == 0 {
		if len(container.Networks) != 0 || len(container.Routes) != 0 {
			return fmt.Errorf("unable to apply network parameters without network namespace")
		}
	} else {
		if err := setupNetwork(container, networkState); err != nil {
			return fmt.Errorf("setup networking %s", err)
		}
		if err := setupRoute(container); err != nil {
			return fmt.Errorf("setup route %s", err)
		}
	}

	label.Init()

	// InitializeMountNamespace() can be executed only for a new mount namespace
	if (cloneFlags & syscall.CLONE_NEWNS) != 0 {
		if err := mount.InitializeMountNamespace(container); err != nil {
			return fmt.Errorf("setup mount namespace %s", err)
		}
	}
	return nil
}

// Finalize entering into a container and execute a specified command
func initIn(pipe *os.File) (err error) {
	defer func() {
		// if we have an error during the initialization of the container's init then send it back to the
		// parent process in the form of an initError.
		if err != nil {
			// ensure that any data sent from the parent is consumed so it doesn't
			// receive ECONNRESET when the child writes to the pipe.
			ioutil.ReadAll(pipe)
			if err := json.NewEncoder(pipe).Encode(initError{
				Message: err.Error(),
			}); err != nil {
				panic(err)
			}
		}
		// ensure that this pipe is always closed
		pipe.Close()
	}()
	decoder := json.NewDecoder(pipe)
	var config *configs.Config
	if err := decoder.Decode(&config); err != nil {
		return err
	}
	var process *processArgs
	if err := decoder.Decode(&process); err != nil {
		return err
	}
	if err := finalizeSetns(config); err != nil {
		return err
	}
	if err := system.Execv(process.Args[0], process.Args[0:], config.Env); err != nil {
		return err
	}
	panic("unreachable")
}

// finalize expects that the setns calls have been setup and that is has joined an
// existing namespace
func finalizeSetns(container *configs.Config) error {
	// clear the current processes env and replace it with the environment defined on the container
	if err := loadContainerEnvironment(container); err != nil {
		return err
	}

	if err := setupRlimits(container); err != nil {
		return fmt.Errorf("setup rlimits %s", err)
	}

	if err := finalizeNamespace(container); err != nil {
		return err
	}

	if err := apparmor.ApplyProfile(container.AppArmorProfile); err != nil {
		return fmt.Errorf("set apparmor profile %s: %s", container.AppArmorProfile, err)
	}

	if container.ProcessLabel != "" {
		if err := label.SetProcessLabel(container.ProcessLabel); err != nil {
			return err
		}
	}

	return nil
}

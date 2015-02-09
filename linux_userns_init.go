// +build linux

package libcontainer

import (
	"syscall"

	"github.com/docker/libcontainer/apparmor"
	consolepkg "github.com/docker/libcontainer/console"
	"github.com/docker/libcontainer/label"
	"github.com/docker/libcontainer/security/restrict"
	"github.com/docker/libcontainer/system"
)

type linuxUsernsInit struct {
	config *initConfig
}

func (l *linuxUsernsInit) Init() error {
	// join any namespaces via a path to the namespace fd if provided
	if err := joinExistingNamespaces(l.config.Config.Namespaces); err != nil {
		return err
	}
	console := l.config.Config.Console
	if console != "" {
		if err := consolepkg.OpenAndDup("/dev/console"); err != nil {
			return err
		}
	}
	if _, err := syscall.Setsid(); err != nil {
		return err
	}
	if console != "" {
		if err := system.Setctty(); err != nil {
			return err
		}
	}
	if l.config.Cwd == "" {
		l.config.Cwd = "/"
	}
	if err := setupRlimits(l.config.Config); err != nil {
		return err
	}
	if hostname := l.config.Config.Hostname; hostname != "" {
		if err := syscall.Sethostname([]byte(hostname)); err != nil {
			return err
		}
	}
	if err := apparmor.ApplyProfile(l.config.Config.AppArmorProfile); err != nil {
		return err
	}
	if err := label.SetProcessLabel(l.config.Config.ProcessLabel); err != nil {
		return err
	}
	if l.config.Config.RestrictSys {
		if err := restrict.Restrict("proc/sys", "proc/sysrq-trigger", "proc/irq", "proc/bus"); err != nil {
			return err
		}
	}
	pdeath, err := system.GetParentDeathSignal()
	if err != nil {
		return err
	}
	if err := finalizeNamespace(l.config); err != nil {
		return err
	}
	// finalizeNamespace can change user/group which clears the parent death
	// signal, so we restore it here.
	if err := pdeath.Restore(); err != nil {
		return err
	}
	// Signal self if parent is already dead. Does nothing if running in a new
	// PID namespace, as Getppid will always return 0.
	if syscall.Getppid() == 1 {
		return syscall.Kill(syscall.Getpid(), syscall.SIGKILL)
	}
	return system.Execv(l.config.Args[0], l.config.Args[0:], l.config.Env)
}

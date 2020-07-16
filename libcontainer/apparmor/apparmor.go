// +build apparmor,linux

package apparmor

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/opencontainers/runc/libcontainer/utils"
)

// IsEnabled returns true if apparmor is enabled for the host.
func IsEnabled() bool {
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err == nil && os.Getenv("container") == "" {
		if _, err = os.Stat("/sbin/apparmor_parser"); err == nil {
			buf, err := ioutil.ReadFile("/sys/module/apparmor/parameters/enabled")
			return err == nil && len(buf) > 1 && buf[0] == 'Y'
		}
	}
	return false
}

func setProcAttr(attr, value string, useThread bool) error {
	// Under AppArmor you can only change your own attr, so use /proc/self/
	// instead of /proc/<tid>/ like libapparmor does
	var path string
	if useThread {
		path = fmt.Sprintf("/proc/thread-self/attr/%s", attr)
	} else {
		path = fmt.Sprintf("/proc/self/attr/%s", attr)
	}

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := utils.EnsureProcHandle(f); err != nil {
		return err
	}

	_, err = fmt.Fprintf(f, "%s", value)
	return err
}

// changeOnExec reimplements aa_change_onexec from libapparmor in Go
func changeOnExec(name string, useThread bool) error {
	value := "exec " + name
	if err := setProcAttr("exec", value, useThread); err != nil {
		return fmt.Errorf("apparmor failed to apply profile: %s", err)
	}
	return nil
}

// ApplyProfileThread will apply the profile with the specified name to the process
// after the next exec using /proc/self-thread rather than /proc/self
func ApplyProfileThread(name string) error {
	if name == "" {
		return nil
	}
	return changeOnExec(name, true)
}

// ApplyProfile will apply the profile with the specified name to the process after
// the next exec.
func ApplyProfile(name string) error {
	if name == "" {
		return nil
	}

	return changeOnExec(name, false)
}

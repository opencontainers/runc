package apparmor

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/internal/pathrs"
)

// isEnabled returns true if apparmor is enabled for the host.
var isEnabled = sync.OnceValue(func() bool {
	if _, err := os.Stat("/sys/kernel/security/apparmor"); err != nil {
		return false
	}
	buf, err := os.ReadFile("/sys/module/apparmor/parameters/enabled")
	return err == nil && len(buf) > 0 && buf[0] == 'Y'
})

// changeOnExec reimplements aa_change_onexec from libapparmor in Go.
func changeOnExec(name string) error {
	attrSubPath := "attr/apparmor/exec"
	if _, err := os.Stat("/proc/self/" + attrSubPath); errors.Is(err, os.ErrNotExist) {
		// fall back to the old convention
		attrSubPath = "attr/exec"
	}

	// Under AppArmor you can only change your own attr, so there's no reason
	// to not use /proc/thread-self/ (instead of /proc/<tid>/, like libapparmor
	// does).
	f, closer, err := pathrs.ProcThreadSelfOpen(attrSubPath, unix.O_WRONLY|unix.O_CLOEXEC)
	if err != nil {
		return err
	}
	defer closer()
	defer f.Close()

	_, err = f.WriteString("exec " + name)
	if errors.Is(err, unix.ENOENT) {
		// ENOENT from write here is AppArmor telling the profile is unknown.
		err = errors.New("profile not loaded")
	}
	return err
}

// applyProfile will apply the profile with the specified name to the process
// after the next exec.
func applyProfile(name string) error {
	if name == "" {
		return nil
	}

	err := changeOnExec(name)
	if err != nil {
		return fmt.Errorf("unable to apply apparmor profile %q: %w", name, err)
	}
	return nil
}

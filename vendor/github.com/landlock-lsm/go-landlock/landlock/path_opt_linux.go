//go:build linux

package landlock

import (
	"errors"
	"fmt"
	"syscall"

	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

func (r FSRule) addToRuleset(rulesetFD int, c Config) error {
	effectiveAccessFS := r.accessFS
	if !r.enforceSubset {
		effectiveAccessFS = effectiveAccessFS.intersect(c.handledAccessFS)
	}
	if effectiveAccessFS == 0 {
		// Adding this to the ruleset would be a no-op
		// and result in an error.
		return nil
	}
	for _, path := range r.paths {
		if err := addPath(rulesetFD, path, effectiveAccessFS); err != nil {
			if r.ignoreMissing && errors.Is(err, unix.ENOENT) {
				continue // Skip this path.
			}
			return fmt.Errorf("populating ruleset for %q with access %v: %w", path, effectiveAccessFS, err)
		}
	}
	return nil
}

func addPath(rulesetFd int, path string, access AccessFSSet) error {
	fd, err := syscall.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer syscall.Close(fd)

	pathBeneath := ll.PathBeneathAttr{
		ParentFd:      fd,
		AllowedAccess: uint64(access),
	}
	err = ll.LandlockAddPathBeneathRule(rulesetFd, &pathBeneath, 0)
	if err != nil {
		if errors.Is(err, syscall.EINVAL) {
			// The ruleset access permissions must be a superset of the ones we restrict to.
			// This should never happen because the call to addPath() ensures that.
			err = fmt.Errorf("inconsistent access rights (using directory access rights on a regular file?): %w", err)
		} else if errors.Is(err, syscall.ENOMSG) && access == 0 {
			err = fmt.Errorf("empty access rights: %w", err)
		} else {
			// Other errors should never happen.
			err = bug(err)
		}
		return fmt.Errorf("landlock_add_rule: %w", err)
	}
	return nil
}

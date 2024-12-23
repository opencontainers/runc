package landlock

import (
	"errors"
	"fmt"
	"syscall"

	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// The actual restrictPaths implementation.
func restrictPaths(c Config, opts ...PathOpt) error {
	handledAccessFS := c.handledAccessFS
	abi := getSupportedABIVersion()
	if c.bestEffort {
		handledAccessFS = handledAccessFS.intersect(abi.supportedAccessFS)
	} else {
		if !handledAccessFS.isSubset(abi.supportedAccessFS) {
			return fmt.Errorf("missing kernel Landlock support. Got Landlock ABI v%v, wanted %v", abi.version, c.String())
		}
	}
	if handledAccessFS.isEmpty() {
		return nil // Success: Nothing to restrict.
	}

	rulesetAttr := ll.RulesetAttr{
		HandledAccessFS: uint64(handledAccessFS),
	}
	fd, err := ll.LandlockCreateRuleset(&rulesetAttr, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOSYS) || errors.Is(err, syscall.EOPNOTSUPP) {
			err = errors.New("landlock is not supported by kernel or not enabled at boot time")
		}
		if errors.Is(err, syscall.EINVAL) {
			err = errors.New("unknown flags, unknown access, or too small size")
		}
		// Bug, because these should have been caught up front with the ABI version check.
		return bug(fmt.Errorf("landlock_create_ruleset: %w", err))
	}
	defer syscall.Close(fd)

	for _, opt := range opts {
		accessFS := opt.accessFS.intersect(handledAccessFS)
		if err := populateRuleset(fd, opt.paths, accessFS); err != nil {
			return err
		}
	}

	if err := ll.AllThreadsPrctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		// This prctl invocation should always work.
		return bug(fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %v", err))
	}

	if err := ll.AllThreadsLandlockRestrictSelf(fd, 0); err != nil {
		if errors.Is(err, syscall.E2BIG) {
			// Other errors than E2BIG should never happen.
			return fmt.Errorf("the maximum number of stacked rulesets is reached for the current thread: %w", err)
		}
		return bug(fmt.Errorf("landlock_restrict_self: %w", err))
	}
	return nil
}

func populateRuleset(rulesetFd int, paths []string, access AccessFSSet) error {
	for _, p := range paths {
		if err := populate(rulesetFd, p, access); err != nil {
			return fmt.Errorf("populating ruleset for %q with access %v: %w", p, access, err)
		}
	}
	return nil
}

func populate(rulesetFd int, path string, access AccessFSSet) error {
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
			// This should never happen because the call to populate() ensures that.
			err = bug(fmt.Errorf("invalid flags, or inconsistent access in the rule: %w", err))
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

// Denotes an error that should not have happened.
// If such an error occurs anyway, please try upgrading the library
// and file a bug to github.com/landlock-lsm/go-landlock if the issue persists.
func bug(err error) error {
	return fmt.Errorf("BUG(go-landlock): This should not have happened: %w", err)
}

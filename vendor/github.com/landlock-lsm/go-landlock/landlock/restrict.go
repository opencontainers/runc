//go:build linux

package landlock

import (
	"errors"
	"fmt"
	"syscall"

	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
	"golang.org/x/sys/unix"
)

// downgrade calculates the actual ruleset to be enforced given the
// current kernel's Landlock ABI level.
//
// It establishes that rule.compatibleWithConfig(c) and c.compatibleWithABI(abi).
func downgrade(c Config, rules []Rule, abi abiInfo) (Config, []Rule) {
	c = c.restrictTo(abi)

	resRules := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		rule, ok := rule.downgrade(c)
		if !ok {
			return v0, nil // Use "ABI V0" (do nothing)
		}
		resRules = append(resRules, rule)
	}
	return c, resRules
}

// restrict is the actual implementation which sets up Landlock.
func restrict(c Config, rules ...Rule) error {
	// Check validity of rules early.
	for _, rule := range rules {
		if !rule.compatibleWithConfig(c) {
			return fmt.Errorf("incompatible rule %v: %w", rule, unix.EINVAL)
		}
	}

	abi := getSupportedABIVersion()
	if c.bestEffort {
		c, rules = downgrade(c, rules, abi)
	}
	if !c.compatibleWithABI(abi) {
		return fmt.Errorf("missing kernel Landlock support. Got Landlock ABI v%v, wanted %v", abi.version, c)
	}

	// TODO: This might be incorrect - the "refer" permission is
	// always implicit, even in Landlock V1. So enabling Landlock
	// on a Landlock V1 kernel without any handled access rights
	// will still forbid linking files between directories.
	if c.handledAccessFS.isEmpty() && c.handledAccessNet.isEmpty() {
		return nil // Success: Nothing to restrict.
	}

	rulesetAttr := ll.RulesetAttr{
		HandledAccessFS:  uint64(c.handledAccessFS),
		HandledAccessNet: uint64(c.handledAccessNet),
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

	for _, rule := range rules {
		if err := rule.addToRuleset(fd, c); err != nil {
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

// Denotes an error that should not have happened.
// If such an error occurs anyway, please try upgrading the library
// and file a bug to github.com/landlock-lsm/go-landlock if the issue persists.
func bug(err error) error {
	return fmt.Errorf("BUG(go-landlock): This should not have happened: %w", err)
}

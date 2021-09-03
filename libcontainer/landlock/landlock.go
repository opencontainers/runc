package landlock

import (
	"errors"
	"fmt"

	"github.com/landlock-lsm/go-landlock/landlock"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// Initialize Landlock unprivileged access control for the container process
// based on the given settings.
// The specified `ruleset` identifies a set of rules (i.e., actions on objects)
// that need to be handled (i.e., restricted) by Landlock. And if no `rule`
// explicitly allow them, they should then be forbidden.
// The `disableBestEffort` input gives control over whether the best-effort
// security approach should be applied for Landlock access rights.
func InitLandlock(config *configs.Landlock) error {
	if config == nil {
		return errors.New("cannot initialize Landlock - nil config passed")
	}

	var llConfig landlock.Config

	ruleset := config.Ruleset.HandledAccessFS
	// Panic on error when constructing the Landlock configuration using invalid config values.
	if config.DisableBestEffort {
		llConfig = landlock.MustConfig(ruleset)
	} else {
		llConfig = landlock.MustConfig(ruleset).BestEffort()
	}

	if err := llConfig.RestrictPaths(
		pathAccesses(config.Rules)...,
	); err != nil {
		return fmt.Errorf("could not restrict paths: %w", err)
	}

	return nil
}

// Convert Libcontainer RulePathBeneath to go-landlock PathOpt.
func pathAccess(rule *configs.RulePathBeneath) landlock.PathOpt {
	return landlock.PathAccess(rule.AllowedAccess, rule.Paths...)
}

// Convert Libcontainer Rules to an array of go-landlock PathOpt.
func pathAccesses(rules *configs.Rules) []landlock.PathOpt {
	pathAccesses := []landlock.PathOpt{}

	for _, rule := range rules.PathBeneath {
		opt := pathAccess(rule)
		pathAccesses = append(pathAccesses, opt)
	}

	return pathAccesses
}

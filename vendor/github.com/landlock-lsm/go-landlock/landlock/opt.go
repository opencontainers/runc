package landlock

// Rule represents one or more Landlock rules which can be added to a
// Landlock ruleset.
type Rule interface {
	// compatibleWithConfig is true if the given rule is
	// compatible with the configuration c.
	compatibleWithConfig(c Config) bool

	// downgrade returns a downgraded rule for "best effort" mode,
	// under the assumption that the kernel only supports c.
	//
	// It establishes that:
	//
	//   - rule.accessFS ⊆ handledAccessFS for FSRules
	//   - rule.accessNet ⊆ handledAccessNet for NetRules
	//
	// If the rule is unsupportable under the given Config at
	// all, ok is false. This happens when c represents a Landlock
	// V1 system but the rule wants to grant the refer right on
	// a path. "Refer" operations are always forbidden under
	// Landlock V1.
	downgrade(c Config) (out Rule, ok bool)

	// addToRuleset applies the rule to the given rulesetFD.
	//
	// This may return errors such as "file not found" depending
	// on the rule type.
	addToRuleset(rulesetFD int, c Config) error
}

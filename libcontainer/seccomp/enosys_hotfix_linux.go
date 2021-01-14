// +build linux,cgo,seccomp

package seccomp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
	libseccomp "github.com/seccomp/libseccomp-golang"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func baseAction(act libseccomp.ScmpAction) configs.Action {
	switch act.SetReturnCode(0) {
	case libseccomp.ActAllow:
		return configs.Allow
	case libseccomp.ActTrap:
		return configs.Trap
	case libseccomp.ActKill:
		return configs.Kill
	case libseccomp.ActTrace:
		return configs.Trace
	case libseccomp.ActLog:
		return configs.Log
	case libseccomp.ActErrno:
		return configs.Errno
	}
	panic("unknown base action")
}

func argString(rule configs.Arg) string {
	opFmt, ok := map[configs.Operator]string{
		configs.EqualTo:              "== %d {%#x}",
		configs.NotEqualTo:           "!= %d {%#x}",
		configs.GreaterThan:          "> %d {%#x}",
		configs.GreaterThanOrEqualTo: ">= %d {%#x}",
		configs.LessThan:             "< %d {%#x}",
		configs.LessThanOrEqualTo:    "<= %d {%#x}",
		configs.MaskEqualTo:          "& %#x == %#x",
	}[rule.Op]
	if !ok {
		opFmt = fmt.Sprintf("[unknown op %d] {%%#x %%#x}", rule.Op)
	}
	op := fmt.Sprintf(opFmt, rule.Value, rule.ValueTwo)
	return fmt.Sprintf("arg%d %s", rule.Index, op)
}

func argSetMode(rules []*configs.Arg) (andMode bool) {
	argCounts := map[uint]bool{}
	for _, rule := range rules {
		if argCounts[rule.Index] {
			return false
		}
		argCounts[rule.Index] = true
	}
	return true
}

func argSetString(rules []*configs.Arg, noCompat bool) string {
	// Is the rule set in AND or OR mode?
	op := "&&"
	if !noCompat && !argSetMode(rules) {
		op = "||"
	}
	var ruleString strings.Builder
	for idx, rule := range rules {
		if idx == 0 {
			fmt.Fprintf(&ruleString, "(%s)", argString(*rule))
		} else {
			fmt.Fprintf(&ruleString, " %s (%s)", op, argString(*rule))
		}
	}
	str := ruleString.String()
	if str == "" {
		str = "[always true]"
	}
	return str
}

// Generate a set of rules which operate as the inverse of the given filter
// rule. Most rules only have one corresponding negated rule, the exception is
// SCMP_CMP_MASKED_EQ.
func inverseCondRuleSingle(rule configs.Arg) ([]*configs.Arg, error) {
	// Simple case is when there is an obvious inverse mapping for the rule --
	// in that case we just generate a new rule with the flipped operator.
	invOp, ok := map[configs.Operator]configs.Operator{
		configs.EqualTo:              configs.NotEqualTo,           // (==) -> (!=)
		configs.NotEqualTo:           configs.EqualTo,              // (!=) -> (==)
		configs.GreaterThan:          configs.LessThanOrEqualTo,    // (> ) -> (<=)
		configs.GreaterThanOrEqualTo: configs.LessThan,             // (>=) -> (< )
		configs.LessThan:             configs.GreaterThanOrEqualTo, // (< ) -> (>=)
		configs.LessThanOrEqualTo:    configs.GreaterThan,          // (<=) -> (> )
		// configs.MaskEqualTo doesn't have an obvious translation.
	}[rule.Op]
	if ok {
		invRule := configs.Arg{
			Index:    rule.Index,
			Value:    rule.Value,
			ValueTwo: rule.ValueTwo,
			Op:       invOp,
		}
		logrus.Debugf("\t\t\ttrivial invert (%s) -> (%s)", argString(rule), argString(invRule))
		return []*configs.Arg{&invRule}, nil
	}

	// Only MaskEqualTo should reach here.
	if rule.Op != configs.MaskEqualTo {
		return nil, fmt.Errorf("cannot invert unknown seccomp operator: %v", rule.Op)
	}

	// We generate a series of OR rules to invert MASKED_EQ. Sadly we can't
	// implement this incredibly efficiently, so we need to implement it by
	// generating a series of MASKED_EQs.
	var rules []*configs.Arg
	logrus.Debugf("\t\t\tnon-trivial invert of (%s)", argString(rule))
	if err := utils.BitPowerSet(rule.Value, func(valueTwo uint64) error {
		if valueTwo == rule.ValueTwo {
			// This is the rule we are inverting, so do not include it.
			return nil
		}
		invRule := configs.Arg{
			Index:    rule.Index,
			Value:    rule.Value,
			ValueTwo: valueTwo,
			Op:       configs.MaskEqualTo,
		}
		logrus.Debugf("\t\t\t\t... -> (%s)", argString(invRule))
		rules = append(rules, &invRule)
		return nil
	}); err != nil {
		return nil, err
	}
	return rules, nil
}

// inverseCondRule generates the set of inverse conditional rules for a given
// conditional rule set. Because certain argument settings can be converted to
// multiple rules (if the same argument is listed more than once), we have to
// generate more than one rule in certain scenarios.
func inverseCondRules(args []*configs.Arg) ([][]*configs.Arg, bool, error) {
	// Are we in AND mode or OR mode for the arguments?
	if !argSetMode(args) {
		// If the rule is in OR mode, we would have to generate inverse AND
		// rules. This is all well and good, but unfortunately since the only
		// time we will be in OR mode is if two rules match the same argument,
		// we cannot do this due to a libseccomp limitation.
		//
		// So we have to simply bail and let this case be treated as -ENOSYS.
		return nil, false, nil
	}

	// Compute the inverted rule set.
	invRulesSet := [][]*configs.Arg{}
	for _, arg := range args {
		invArgs, err := inverseCondRuleSingle(*arg)
		if err != nil {
			return nil, false, err
		}
		// If the rule is in AND mode, generate inverse OR rules. If
		// the inverse rules are OR rules, we treat them as separate OR
		// rules.
		for _, invArg := range invArgs {
			invRulesSet = append(invRulesSet, []*configs.Arg{invArg})
		}
	}
	return invRulesSet, true, nil
}

// enosysHotfix updates the configuration such that it is more friendly to
// glibc and other programs that depend on syscalls returning -ENOSYS rather
// than -EPERM for fallback behaviour.
//
// We do not use the configured default action in certain circumstances --
// namely, if the default action is SET_ERRNO or SET_TRACE we force the default
// action to have an errno of -ENOSYS and then generate specialised -EPERM
// rules for all "known to be valid" syscalls. The "known to be valid" syscalls
// are basically all those which existed in Linux 3.0 and earlier (cut-off
// version is arbitrary -- it's the oldest RHEL LTSS kernel at time of
// writing).
//
// We also make a best effort to generate inverse rules for any conditional
// seccomp rules, the idea being that we want to avoid giving -ENOSYS for
// something which is intentionally meant to be an -EPERM. Sadly there are
// pretty big libseccomp limitations which stop this from being done in all
// cases.
//
// FIXME: Once libseccomp supports minimum-kernel-version specifications, we
//        should be able to remove most of these workarounds by adding a
//        minimum kernel version field to the runtime-spec.
//
// See <https://github.com/opencontainers/runc/issues/2151> and
// <https://github.com/seccomp/libseccomp/issues/286> for more detail on how
// this may be solved more elegantly in the future.
func enosysHotfix(filter *libseccomp.ScmpFilter, config *configs.Seccomp) error {
	oldAction := config.DefaultAction
	oldErrno := uint(unix.EPERM) // hardcoded

	if oldAction != configs.Errno {
		// We don't care about any of these workarounds if the default action
		// doesn't touch the errno.
		return nil
	}

	logrus.Debugf("seccomp: installing -ENOSYS workaround rules")

	// Track which syscalls appear more than once. Any syscall which appears
	// more than once is "too hard" and gets the -ENOSYS treatment. This is the
	// same as auto-generated OR rules.
	countSyscalls := map[string]int{}
	complexSyscalls := map[string]bool{}
	for _, rule := range config.Syscalls {
		countSyscalls[rule.Name] += 1
		// Allow rules are different to deny rules in that deny rules become
		// AND rules, while allow rules are OR rules (a syscall will be
		// permitted if any allow rule passes but will be blocked if any deny
		// rule passes). In order to avoid accidentally making a rule insecure,
		// we only hotfix syscalls which have a single allow rule.
		switch rule.Action {
		case configs.Allow, configs.Log:
		default:
			complexSyscalls[rule.Name] = true
		}
	}

	// For each syscall in the ruleset, create inverse rules with an -EPERM
	// action to match intended behaviour.
	for _, rule := range config.Syscalls {
		logrus.Debugf("\thit %s (%s) [%s errno=%d] rule",
			rule.Name, argSetString(rule.Args, false), rule.Action.String(), rule.ErrnoRet)

		if countSyscalls[rule.Name] > 1 || complexSyscalls[rule.Name] {
			// Too hard, just let it get -ENOSYS'd.
			logrus.Debugf("\t\trule set too complicated -- skipping")
			continue
		}

		if len(rule.Args) == 0 {
			// Skip unconditional syscall rules.
			continue
		}

		// Make inverse rules.
		inverseRules, possible, err := inverseCondRules(rule.Args)
		if err != nil {
			return err
		}
		if !possible {
			// AND rules for a single argument cannot be applied by libseccomp
			// at the moment, so we just permit -ENOSYS in this one case.
			logrus.Debugf("\t\trule set cannot be inverted -- skipping")
			continue
		}
		for _, inverseRule := range inverseRules {
			logrus.Debugf("\t\tadd filter %s (%s) => [%s errno=%d]",
				rule.Name, argSetString(inverseRule, true), oldAction.String(), oldErrno)
			if err := matchCallInternal(filter, &configs.Syscall{
				Name:     rule.Name,
				Action:   oldAction,
				ErrnoRet: &oldErrno,
				Args:     inverseRule,
			}); err != nil {
				return err
			}
		}
	}

	// And now we create unconditional rules for any syscalls not present in
	// the allow list at all, up to the last syscall number (which is currently
	// the last syscall added to Linux 3.0 -- "setns").
	lastSysNo, err := libseccomp.GetSyscallFromName("setns")
	if err != nil {
		return errors.New("ENOSYS seccomp workaround: cannot find syscall number for 'setns'")
	}
	for sysNo := 0; sysNo <= int(lastSysNo); sysNo++ {
		sysName, err := libseccomp.ScmpSyscall(sysNo).GetName()
		if err != nil {
			// No such syscall...
			continue
		}
		if _, ok := countSyscalls[sysName]; ok {
			// Rule already created.
			continue
		}
		logrus.Debugf("\tadd blanket -EPERM rule %s => [%s errno=%d]",
			sysName, oldAction.String(), oldErrno)
		if err := matchCallInternal(filter, &configs.Syscall{
			Name:     sysName,
			Action:   oldAction,
			ErrnoRet: &oldErrno,
		}); err != nil {
			return err
		}
	}

	logrus.Debugf("seccomp: completed -ENOSYS workaround rules")
	return nil
}

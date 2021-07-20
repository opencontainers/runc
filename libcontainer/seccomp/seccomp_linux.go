// +build linux,cgo,seccomp

package seccomp

import (
	"errors"
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/seccomp/patchbpf"

	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/sys/unix"
)

var (
	actAllow  = libseccomp.ActAllow
	actTrap   = libseccomp.ActTrap
	actKill   = libseccomp.ActKill
	actTrace  = libseccomp.ActTrace.SetReturnCode(int16(unix.EPERM))
	actLog    = libseccomp.ActLog
	actErrno  = libseccomp.ActErrno.SetReturnCode(int16(unix.EPERM))
	actNotify = libseccomp.ActNotify
)

const (
	// Linux system calls can have at most 6 arguments
	syscallMaxArguments int = 6
)

// InitSeccomp installs the seccomp filters to be used in the container as
// specified in config.
// Returns the seccomp file descriptor if any of the filters include a
// SCMP_ACT_NOTIFY action, otherwise returns -1.
func InitSeccomp(config *configs.Seccomp) (int, error) {
	if config == nil {
		return -1, errors.New("cannot initialize Seccomp - nil config passed")
	}

	defaultAction, err := getAction(config.DefaultAction, config.DefaultErrnoRet)
	if err != nil {
		return -1, errors.New("error initializing seccomp - invalid default action")
	}

	if defaultAction == actNotify {
		return -1, errors.New("SCMP_ACT_NOTIFY cannot be used as default action")
	}

	filter, err := libseccomp.NewFilter(defaultAction)
	if err != nil {
		return -1, fmt.Errorf("error creating filter: %w", err)
	}

	// TODO: config.Flags defines the options to pass to seccomp(2) but
	// it's not taken into consideration.
	for _, call := range config.Syscalls {
		if call.Action == configs.Notify && call.Name == "write" {
			return -1, errors.New("SCMP_ACT_NOTIFY cannot be used for the write syscall")
		}
	}

	// Add extra architectures
	for _, arch := range config.Architectures {
		scmpArch, err := libseccomp.GetArchFromString(arch)
		if err != nil {
			return -1, fmt.Errorf("error validating Seccomp architecture: %w", err)
		}
		if err := filter.AddArch(scmpArch); err != nil {
			return -1, fmt.Errorf("error adding architecture to seccomp filter: %w", err)
		}
	}

	// Unset no new privs bit
	if err := filter.SetNoNewPrivsBit(false); err != nil {
		return -1, fmt.Errorf("error setting no new privileges: %w", err)
	}

	// Add a rule for each syscall
	for _, call := range config.Syscalls {
		if call == nil {
			return -1, errors.New("encountered nil syscall while initializing Seccomp")
		}

		if err = matchCall(filter, call); err != nil {
			return -1, err
		}
	}

	seccompFd, err := patchbpf.PatchAndLoad(config, filter)
	if err != nil {
		return -1, fmt.Errorf("error loading seccomp filter into kernel: %w", err)
	}

	return seccompFd, nil
}

// Convert Libcontainer Action to Libseccomp ScmpAction
func getAction(act configs.Action, errnoRet *uint) (libseccomp.ScmpAction, error) {
	switch act {
	case configs.Kill:
		return actKill, nil
	case configs.Errno:
		if errnoRet != nil {
			return libseccomp.ActErrno.SetReturnCode(int16(*errnoRet)), nil
		}
		return actErrno, nil
	case configs.Trap:
		return actTrap, nil
	case configs.Allow:
		return actAllow, nil
	case configs.Trace:
		if errnoRet != nil {
			return libseccomp.ActTrace.SetReturnCode(int16(*errnoRet)), nil
		}
		return actTrace, nil
	case configs.Log:
		return actLog, nil
	case configs.Notify:
		return actNotify, nil
	default:
		return libseccomp.ActInvalid, errors.New("invalid action, cannot use in rule")
	}
}

// Convert Libcontainer Operator to Libseccomp ScmpCompareOp
func getOperator(op configs.Operator) (libseccomp.ScmpCompareOp, error) {
	switch op {
	case configs.EqualTo:
		return libseccomp.CompareEqual, nil
	case configs.NotEqualTo:
		return libseccomp.CompareNotEqual, nil
	case configs.GreaterThan:
		return libseccomp.CompareGreater, nil
	case configs.GreaterThanOrEqualTo:
		return libseccomp.CompareGreaterEqual, nil
	case configs.LessThan:
		return libseccomp.CompareLess, nil
	case configs.LessThanOrEqualTo:
		return libseccomp.CompareLessOrEqual, nil
	case configs.MaskEqualTo:
		return libseccomp.CompareMaskedEqual, nil
	default:
		return libseccomp.CompareInvalid, errors.New("invalid operator, cannot use in rule")
	}
}

// Convert Libcontainer Arg to Libseccomp ScmpCondition
func getCondition(arg *configs.Arg) (libseccomp.ScmpCondition, error) {
	cond := libseccomp.ScmpCondition{}

	if arg == nil {
		return cond, errors.New("cannot convert nil to syscall condition")
	}

	op, err := getOperator(arg.Op)
	if err != nil {
		return cond, err
	}

	return libseccomp.MakeCondition(arg.Index, op, arg.Value, arg.ValueTwo)
}

// Add a rule to match a single syscall
func matchCall(filter *libseccomp.ScmpFilter, call *configs.Syscall) error {
	if call == nil || filter == nil {
		return errors.New("cannot use nil as syscall to block")
	}

	if len(call.Name) == 0 {
		return errors.New("empty string is not a valid syscall")
	}

	// If we can't resolve the syscall, assume it's not supported on this kernel
	// Ignore it, don't error out
	callNum, err := libseccomp.GetSyscallFromName(call.Name)
	if err != nil {
		return nil
	}

	// Convert the call's action to the libseccomp equivalent
	callAct, err := getAction(call.Action, call.ErrnoRet)
	if err != nil {
		return fmt.Errorf("action in seccomp profile is invalid: %w", err)
	}

	// Unconditional match - just add the rule
	if len(call.Args) == 0 {
		if err := filter.AddRule(callNum, callAct); err != nil {
			return fmt.Errorf("error adding seccomp filter rule for syscall %s: %w", call.Name, err)
		}
	} else {
		// If two or more arguments have the same condition,
		// Revert to old behavior, adding each condition as a separate rule
		argCounts := make([]uint, syscallMaxArguments)
		conditions := []libseccomp.ScmpCondition{}

		for _, cond := range call.Args {
			newCond, err := getCondition(cond)
			if err != nil {
				return fmt.Errorf("error creating seccomp syscall condition for syscall %s: %w", call.Name, err)
			}

			argCounts[cond.Index] += 1

			conditions = append(conditions, newCond)
		}

		hasMultipleArgs := false
		for _, count := range argCounts {
			if count > 1 {
				hasMultipleArgs = true
				break
			}
		}

		if hasMultipleArgs {
			// Revert to old behavior
			// Add each condition attached to a separate rule
			for _, cond := range conditions {
				condArr := []libseccomp.ScmpCondition{cond}

				if err := filter.AddRuleConditional(callNum, callAct, condArr); err != nil {
					return fmt.Errorf("error adding seccomp rule for syscall %s: %w", call.Name, err)
				}
			}
		} else {
			// No conditions share same argument
			// Use new, proper behavior
			if err := filter.AddRuleConditional(callNum, callAct, conditions); err != nil {
				return fmt.Errorf("error adding seccomp rule for syscall %s: %w", call.Name, err)
			}
		}
	}

	return nil
}

// Version returns major, minor, and micro.
func Version() (uint, uint, uint) {
	return libseccomp.GetLibraryVersion()
}

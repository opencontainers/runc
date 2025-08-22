//go:build linux

package syscall

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
	"kernel.org/pub/linux/libs/security/libcap/psx"
)

// LandlockCreateRuleset creates a ruleset file descriptor with the
// given attributes.
func LandlockCreateRuleset(attr *RulesetAttr, flags int) (fd int, err error) {
	r0, _, e1 := syscall.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, uintptr(unsafe.Pointer(attr)), uintptr(rulesetAttrSize), uintptr(flags))
	fd = int(r0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// LandlockGetABIVersion returns the supported Landlock ABI version (starting at 1).
func LandlockGetABIVersion() (version int, err error) {
	r0, _, e1 := syscall.Syscall(unix.SYS_LANDLOCK_CREATE_RULESET, 0, 0, unix.LANDLOCK_CREATE_RULESET_VERSION)
	version = int(r0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// Landlock rule types.
const (
	RuleTypePathBeneath = unix.LANDLOCK_RULE_PATH_BENEATH
	RuleTypeNetPort     = 2 // TODO: Use it from sys/unix when available.
)

// LandlockAddPathBeneathRule adds a rule of type "path beneath" to
// the given ruleset fd. attr defines the rule parameters. flags must
// currently be 0.
func LandlockAddPathBeneathRule(rulesetFd int, attr *PathBeneathAttr, flags int) error {
	return LandlockAddRule(rulesetFd, RuleTypePathBeneath, unsafe.Pointer(attr), flags)
}

// LandlockAddNetPortRule adds a rule of type "net port" to the given ruleset FD.
// attr defines the rule parameters. flags must currently be 0.
func LandlockAddNetPortRule(rulesetFD int, attr *NetPortAttr, flags int) error {
	return LandlockAddRule(rulesetFD, RuleTypeNetPort, unsafe.Pointer(attr), flags)
}

// LandlockAddRule is the generic landlock_add_rule syscall.
func LandlockAddRule(rulesetFd int, ruleType int, ruleAttr unsafe.Pointer, flags int) (err error) {
	_, _, e1 := syscall.Syscall6(unix.SYS_LANDLOCK_ADD_RULE, uintptr(rulesetFd), uintptr(ruleType), uintptr(ruleAttr), uintptr(flags), 0, 0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// AllThreadsLandlockRestrictSelf enforces the given ruleset on all OS
// threads belonging to the current process.
func AllThreadsLandlockRestrictSelf(rulesetFd int, flags int) (err error) {
	_, _, e1 := psx.Syscall3(unix.SYS_LANDLOCK_RESTRICT_SELF, uintptr(rulesetFd), uintptr(flags), 0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

// AllThreadsPrctl is like unix.Prctl, but gets applied on all OS threads at the same time.
func AllThreadsPrctl(option int, arg2, arg3, arg4, arg5 uintptr) (err error) {
	_, _, e1 := psx.Syscall6(syscall.SYS_PRCTL, uintptr(option), uintptr(arg2), uintptr(arg3), uintptr(arg4), uintptr(arg5), 0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}

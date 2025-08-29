//go:build !linux

package syscall

import (
	"syscall"
	"unsafe"
)

func LandlockCreateRuleset(attr *RulesetAttr, flags int) (fd int, err error) {
	return -1, syscall.ENOSYS
}

func LandlockGetABIVersion() (version int, err error) {
	return -1, syscall.ENOSYS
}

func LandlockAddRule(rulesetFd int, ruleType int, ruleAttr unsafe.Pointer, flags int) (err error) {
	return syscall.ENOSYS
}

func LandlockAddPathBeneathRule(rulesetFd int, attr *PathBeneathAttr, flags int) error {
	return syscall.ENOSYS
}

func LandlockAddNetPortRule(rulesetFD int, attr *NetPortAttr, flags int) error {
	return syscall.ENOSYS
}

func AllThreadsLandlockRestrictSelf(rulesetFd int, flags int) (err error) {
	return syscall.ENOSYS
}

func AllThreadsPrctl(option int, arg2, arg3, arg4, arg5 uintptr) (err error) {
	return syscall.ENOSYS
}

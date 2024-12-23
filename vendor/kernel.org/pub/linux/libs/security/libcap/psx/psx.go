// +build linux,!cgo
// +build go1.16

package psx // import "kernel.org/pub/linux/libs/security/libcap/psx"

import "syscall"

// Documentation for these functions are provided in the psx_cgo.go
// file.

//go:uintptrescapes
func Syscall3(syscallnr, arg1, arg2, arg3 uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.AllThreadsSyscall(syscallnr, arg1, arg2, arg3)
}

//go:uintptrescapes
func Syscall6(syscallnr, arg1, arg2, arg3, arg4, arg5, arg6 uintptr) (uintptr, uintptr, syscall.Errno) {
	return syscall.AllThreadsSyscall6(syscallnr, arg1, arg2, arg3, arg4, arg5, arg6)
}

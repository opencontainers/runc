//go:build go1.21

package nsenter

// Since Go 1.21 <https://github.com/golang/go/commit/c426c87012b5e>, the Go
// runtime will try to call pthread_getattr_np(pthread_self()). This causes
// issues with nsexec and requires some kludges to overwrite the internal
// thread-local glibc cache of the current TID. See find_glibc_tls_tid_address
// for the horrific details.

// #cgo CFLAGS: -DRUNC_GLIBC_TID_KLUDGE=1
import "C"

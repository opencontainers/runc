//go:build linux && !gccgo

// Package nsenter implements the namespace creation and joining logic of runc.
//
// This package registers a special CGo constructor that will run before the Go
// runtime boots in order to provide a mechanism for runc to operate on
// namespaces that require single-threaded program execution to work.
package nsenter

/*
#cgo CFLAGS: -Wall
extern void nsexec();
void __attribute__((constructor)) init(void) {
	nsexec();
}
*/
import "C"

// +build linux

package nsenter

/*
#cgo CFLAGS: -Wall
extern void nsexec();
void __attribute__((constructor)) init() {
	nsexec();
}
*/
import "C"

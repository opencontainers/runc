// +build linux,!gccgo

package nsenter

/*
#cgo CFLAGS: -Wall
extern void nsexec();
void __attribute__((constructor)) init(int argc, char *argv[]) {
	nsexec(argv);
}
*/
import "C"

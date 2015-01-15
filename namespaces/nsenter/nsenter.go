// +build linux

package nsenter

/*
__attribute__((constructor)) init() {
	nsexec();
}
*/
import "C"

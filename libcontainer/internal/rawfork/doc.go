// Package rawfork runs a simple program of raw system calls in a forked child
// process.
//
// Some operations, such as joining a user or a mount namespace (see setns(2)),
// require a single-threaded process, and thus can not be performed from Go
// code, which is always multithreaded. A forked child is single-threaded, but
// it runs under the same restrictions as the code between fork and exec in
// package syscall: it must not allocate memory, grow its stack, take locks, or
// call anything but raw system calls. For this reason, the whole program is
// prepared by the parent in advance, and the child merely interprets it.
package rawfork

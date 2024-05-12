//go:build linux && !gccgo && go1.22
// +build linux,!gccgo,go1.22

package nsenter

/*
#cgo LDFLAGS: -Wl,--wrap=x_cgo_getstackbound
#define _GNU_SOURCE
#include <pthread.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#include "log.h"

typedef uintptr_t uintptr;

// Since Go 1.22 <https://github.com/golang/go/commit/52e17c2>, the
// go runtime will try to call pthread_getattr_np(pthread_self()) in cgo.
// This will cause issues in nsexec with glibc < 2.32. so it requires
// us to provide a backward compatibility with old versions of glibc.
// The core issue of glibc is that, before 2.32, `pthread_getattr_np`
// did not call `__pthread_attr_init (attr)`, we need to init the attr
// in go runtime. Fortunately, cgo exports a function named `x_cgo_getstackbound`,
// we can wrap it in the compile phrase. Please see:
// https://github.com/golang/go/blob/52e17c2/src/runtime/cgo/gcc_stack_unix.c
// Fix me: this hack looks ugly, once we have removed `clone(2)` in nsexec,
// please remember to remove this hack.
void __wrap_x_cgo_getstackbound(uintptr bounds[2])
{
	pthread_attr_t attr;
	void *addr;
	size_t size;
	int err;

#if defined(__GLIBC__) || (defined(__sun) && !defined(__illumos__))
	// pthread_getattr_np is a GNU extension supported in glibc.
	// Solaris is not glibc but does support pthread_getattr_np
	// (and the fallback doesn't work...). Illumos does not.

	// The main change begin
	// After glibc 2.31, there is a `__pthread_attr_init` call in
	// `pthread_getattr_np`, if there is no init for `attr`, it will
	// cause `pthread_attr_destroy` free some unknown memory, which
	// will cause golang crash, so we need to call `pthread_attr_init`
	// firstly to have a backward compatibility with glibc(< 2.32).
	pthread_attr_init(&attr);

	err = pthread_getattr_np(pthread_self(), &attr);  // GNU extension

	// As we all know, when using clone(2), there is a tid dirty cache
	// bug in almost all versions of glibc, which is introduced by:
	// https://sourceware.org/git/?p=glibc.git;a=commitdiff;h=c579f48e
	// But we can ignore this bug because we only need the stack's addr
	// and size here, and the error is from `__pthread_getaffinity_np`,
	// which is unrelated to the stack info.
	if (err != 0 && err != 3) {
		bail("pthread_getattr_np failed: %s", strerror(err));
	}
	// The main change end

	pthread_attr_getstack(&attr, &addr, &size); // low address
#elif defined(__illumos__)
	pthread_attr_init(&attr);
	pthread_attr_get_np(pthread_self(), &attr);
	pthread_attr_getstack(&attr, &addr, &size); // low address
#else
	// We don't know how to get the current stacks, so assume they are the
	// same as the default stack bounds.
	pthread_attr_init(&attr);
	pthread_attr_getstacksize(&attr, &size);
	addr = __builtin_frame_address(0) + 4096 - size;
#endif
	pthread_attr_destroy(&attr);

	bounds[0] = (uintptr)addr;
	bounds[1] = (uintptr)addr + size;
}
*/
import "C"

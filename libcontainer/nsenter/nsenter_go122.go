//go:build go1.22 && !go1.23

package nsenter

/*
// In glibc versions older than 2.32 (before commit 4721f95058),
// pthread_getattr_np does not always initialize the `attr` argument,
// and when it fails, it results in a NULL pointer dereference in
// pthread_attr_destroy down the road. This has been fixed in go 1.22.4.
// We hack this to let runc can work with glibc < 2.32 in go 1.22.x,
// once runc doesn't support 1.22, we can remove this hack.
#cgo LDFLAGS: -Wl,--wrap=pthread_getattr_np
#include <features.h>
#include <pthread.h>
int __real_pthread_getattr_np (pthread_t __th, pthread_attr_t *__attr);
int __wrap_pthread_getattr_np (pthread_t __th, pthread_attr_t *__attr) {
#if __GLIBC__ && defined(__GLIBC_PREREQ)
# if !__GLIBC_PREREQ(2, 32)
	pthread_attr_init(__attr);
# endif
#endif
	return __real_pthread_getattr_np(__th, __attr);
}
*/
import "C"

//go:build go1.22

package nsenter

/*
// We know for sure that glibc has issues with pthread_self() when called from
// Go after nsenter has run. This is likely a more general problem with how we
// ignore the rules in signal-safety(7), and so it's possible musl will also
// have issues, but as this is just a hotfix let's only block glibc builds.
#include <features.h>
#ifdef __GLIBC__
#  error "runc does not currently work properly with Go >=1.22. See <https://github.com/opencontainers/runc/issues/4233>."
#endif
*/
import "C"

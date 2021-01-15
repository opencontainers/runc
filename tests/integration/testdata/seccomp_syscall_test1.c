#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <assert.h>
#include <errno.h>
#include <fcntl.h>
#include <sched.h>

#include <sys/types.h>
#include <sys/socket.h>
#include <sys/syscall.h>

static int exit_code = 0;

/*
 * We need raw wrappers around each syscall so that glibc won't rewrite the
 * errno value when it is returned from the seccomp filter (glibc has a habit
 * of hiding -ENOSYS if possible -- which counters what we're trying to test).
 */
#define raw(name, ...) \
	syscall(SYS_ ## name, ##__VA_ARGS__)

#define syscall_assert(sval, rval)					\
	do {								\
		int L = (sval), R = (rval);				\
		if (L < 0)						\
			L = -errno;					\
		if (L != R) {						\
			printf("syscall_assert(%s == %s) failed: %d != %d\n", #sval, #rval, L, R); \
			exit_code = 32;					\
		}							\
	} while (0)

int main(void)
{
	// Basic permitted syscalls.
	syscall_assert(write(-1, NULL, 0), -EBADF);

	// Basic syscall with masked rules.
	syscall_assert(raw(socket, AF_UNIX, SOCK_STREAM, 0x000), 3);
	syscall_assert(raw(socket, AF_UNIX, SOCK_STREAM, 0x0FF), -EPROTONOSUPPORT);
	syscall_assert(raw(socket, AF_UNIX, SOCK_STREAM, 0x001), 4);
	syscall_assert(raw(socket, AF_UNIX, SOCK_STREAM, 0x100), -EPERM);
	syscall_assert(raw(socket, AF_UNIX, SOCK_STREAM, 0xC00), -EPERM);

	// Multiple arguments with OR rules.
	syscall_assert(raw(process_vm_readv, 100, NULL, 0, NULL, 0, ~0), -EINVAL);
	syscall_assert(raw(process_vm_readv, 9001, NULL, 0, NULL, 0, ~0), -EINVAL);
	syscall_assert(raw(process_vm_readv, 0, NULL, 0, NULL, 0, ~0), -EPERM);
	syscall_assert(raw(process_vm_readv, 0, NULL, 0, NULL, 0, ~0), -EPERM);

	// Multiple arguments with OR rules -- rule is ERRNO(-ENOANO).
	syscall_assert(raw(process_vm_writev, 1337, NULL, 0, NULL, 0, ~0), -ENOANO);
	syscall_assert(raw(process_vm_writev, 2020, NULL, 0, NULL, 0, ~0), -ENOANO);
	syscall_assert(raw(process_vm_writev, 0, NULL, 0, NULL, 0, ~0), -EPERM);
	syscall_assert(raw(process_vm_writev, 0, NULL, 0, NULL, 0, ~0), -EPERM);

	// Multiple arguments with AND rules.
	syscall_assert(raw(kcmp, 0, 1337, 0, 0, 0), -ESRCH);
	syscall_assert(raw(kcmp, 0, 0, 0, 0, 0), -EPERM);
	syscall_assert(raw(kcmp, 500, 1337, 0, 0, 0), -EPERM);
	syscall_assert(raw(kcmp, 500, 500, 0, 0, 0), -EPERM);

	// Multiple rules for the same syscall.
	syscall_assert(raw(dup3, 0, -100, 0xFFFF), -EPERM);
	syscall_assert(raw(dup3, 1, -100, 0xFFFF), -EINVAL);
	syscall_assert(raw(dup3, 2, -100, 0xFFFF), -EPERM);
	syscall_assert(raw(dup3, 3, -100, 0xFFFF), -EINVAL);

	// Explicitly denied syscalls (those in Linux 3.0) get -EPERM.
	syscall_assert(raw(unshare, 0), -EPERM);
	syscall_assert(raw(setns, 0, 0), -EPERM);

	// Out-of-bounds fake syscall.
	syscall_assert(syscall(1000, 0xDEADBEEF, 0xCAFEFEED, 0x1337), -ENOSYS);

	return exit_code;
}

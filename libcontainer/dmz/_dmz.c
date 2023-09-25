#include "_dmz_arch.h"

void __attribute__((weak, noreturn, optimize("Os", "omit-frame-pointer")))
    _start(void)
{
	/* See https://stackoverflow.com/a/28424728/5167443 for the stack layout */
	register long *sp __asm__(SP);
#ifdef SP_ALIGNMENT
	sp = (long *)(((long)sp) & -SP_ALIGNMENT);
#endif
#if defined(__i386__) || defined(__i486__) || defined(__i586__) || defined(__i686__)
	sp += 4;
#elif defined(__ARM_EABI__)
	sp += 2;
#endif
	long argc = *sp;
	char **argv = (void *)(sp + 1);
	char **environ = argv + argc + 1;
	int rc = 127;
	if (argc >= 1)
		rc = my_syscall3(SYS_execve, argv[0], argv, environ);
	my_syscall3(SYS_exit, rc, 0, 0);
	__builtin_unreachable();
}

/* SPDX-License-Identifier: MIT */
/* my_syscall3 definitions were taken from https://github.com/torvalds/linux/blob/v6.5/tools/include/nolibc/arch.h */

#if defined(__x86_64__)
#define SP	"rsp"
#define SP_ALIGNMENT	16
#define SYS_execve	59
#define SYS_exit	60
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	long _ret;                                                            \
	register long _num  __asm__ ("rax") = (num);                          \
	register long _arg1 __asm__ ("rdi") = (long)(arg1);                   \
	register long _arg2 __asm__ ("rsi") = (long)(arg2);                   \
	register long _arg3 __asm__ ("rdx") = (long)(arg3);                   \
	                                                                      \
	__asm__  volatile (                                                   \
		"syscall\n"                                                   \
		: "=a"(_ret)                                                  \
		: "r"(_arg1), "r"(_arg2), "r"(_arg3),                         \
		  "0"(_num)                                                   \
		: "rcx", "r11", "memory", "cc"                                \
	);                                                                    \
	_ret;                                                                 \
})
#elif defined(__i386__) || defined(__i486__) || defined(__i586__) || defined(__i686__)
#define SP	"esp"
#define SP_ALIGNMENT	16
#define SYS_execve	11
#define SYS_exit	1
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	long _ret;                                                            \
	register long _num __asm__ ("eax") = (num);                           \
	register long _arg1 __asm__ ("ebx") = (long)(arg1);                   \
	register long _arg2 __asm__ ("ecx") = (long)(arg2);                   \
	register long _arg3 __asm__ ("edx") = (long)(arg3);                   \
	                                                                      \
	__asm__  volatile (                                                   \
		"int $0x80\n"                                                 \
		: "=a" (_ret)                                                 \
		: "r"(_arg1), "r"(_arg2), "r"(_arg3),                         \
		  "0"(_num)                                                   \
		: "memory", "cc"                                              \
	);                                                                    \
	_ret;                                                                 \
})
#elif defined(__ARM_EABI__)
#define SP	"r13"
#define SP_ALIGNMENT	8
#define SYS_execve	11
#define SYS_exit	1
#if (defined(__THUMBEB__) || defined(__THUMBEL__)) && \
    !defined(NOLIBC_OMIT_FRAME_POINTER)
/* swap r6,r7 needed in Thumb mode since we can't use nor clobber r7 */
#define _NOLIBC_SYSCALL_REG         "r6"
#define _NOLIBC_THUMB_SET_R7        "eor r7, r6\neor r6, r7\neor r7, r6\n"
#define _NOLIBC_THUMB_RESTORE_R7    "mov r7, r6\n"

#else  /* we're in ARM mode */
/* in Arm mode we can directly use r7 */
#define _NOLIBC_SYSCALL_REG         "r7"
#define _NOLIBC_THUMB_SET_R7        ""
#define _NOLIBC_THUMB_RESTORE_R7    ""
#endif
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	register long _num  __asm__(_NOLIBC_SYSCALL_REG) = (num);             \
	register long _arg1 __asm__ ("r0") = (long)(arg1);                    \
	register long _arg2 __asm__ ("r1") = (long)(arg2);                    \
	register long _arg3 __asm__ ("r2") = (long)(arg3);                    \
	                                                                      \
	__asm__  volatile (                                                   \
		_NOLIBC_THUMB_SET_R7                                          \
		"svc #0\n"                                                    \
		_NOLIBC_THUMB_RESTORE_R7                                      \
		: "=r"(_arg1), "=r" (_num)                                    \
		: "r"(_arg1), "r"(_arg2), "r"(_arg3),                         \
		  "r"(_num)                                                   \
		: "memory", "cc", "lr"                                        \
	);                                                                    \
	_arg1;                                                                \
})
#elif defined(__aarch64__)
#define SP	"sp"
#define SP_ALIGNMENT	16
#define SYS_execve	221
#define SYS_exit	93
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	register long _num  __asm__ ("x8") = (num);                           \
	register long _arg1 __asm__ ("x0") = (long)(arg1);                    \
	register long _arg2 __asm__ ("x1") = (long)(arg2);                    \
	register long _arg3 __asm__ ("x2") = (long)(arg3);                    \
	                                                                      \
	__asm__  volatile (                                                   \
		"svc #0\n"                                                    \
		: "=r"(_arg1)                                                 \
		: "r"(_arg1), "r"(_arg2), "r"(_arg3),                         \
		  "r"(_num)                                                   \
		: "memory", "cc"                                              \
	);                                                                    \
	_arg1;                                                                \
})
#elif defined(__mips__) && defined(_ABIO32)
#define SP	"$sp"
#define SP_ALIGNMENT	8
#define SYS_execve	4011
#define SYS_exit	4001
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	register long _num __asm__ ("v0")  = (num);                           \
	register long _arg1 __asm__ ("a0") = (long)(arg1);                    \
	register long _arg2 __asm__ ("a1") = (long)(arg2);                    \
	register long _arg3 __asm__ ("a2") = (long)(arg3);                    \
	register long _arg4 __asm__ ("a3");                                   \
	                                                                      \
	__asm__  volatile (                                                   \
		"addiu $sp, $sp, -32\n"                                       \
		"syscall\n"                                                   \
		"addiu $sp, $sp, 32\n"                                        \
		: "=r"(_num), "=r"(_arg4)                                     \
		: "0"(_num),                                                  \
		  "r"(_arg1), "r"(_arg2), "r"(_arg3)                          \
		: "memory", "cc", "at", "v1", "hi", "lo",                     \
	          "t0", "t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"  \
	);                                                                    \
	_arg4 ? -_num : _num;                                                 \
})
#elif defined(__riscv)
#define SP	"sp"
#define SP_ALIGNMENT	16
#define SYS_execve	221
#define SYS_exit	93
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	register long _num  __asm__ ("a7") = (num);                           \
	register long _arg1 __asm__ ("a0") = (long)(arg1);                    \
	register long _arg2 __asm__ ("a1") = (long)(arg2);                    \
	register long _arg3 __asm__ ("a2") = (long)(arg3);                    \
									      \
	__asm__  volatile (                                                   \
		"ecall\n\t"                                                   \
		: "+r"(_arg1)                                                 \
		: "r"(_arg2), "r"(_arg3),                                     \
		  "r"(_num)                                                   \
		: "memory", "cc"                                              \
	);                                                                    \
	_arg1;                                                                \
})
#elif defined(__s390x__)
#define SP	"r15"
#define SYS_execve	11
#define SYS_exit	1
#define my_syscall3(num, arg1, arg2, arg3)				\
({									\
	register long _num __asm__ ("1") = (num);			\
	register long _arg1 __asm__ ("2") = (long)(arg1);		\
	register long _arg2 __asm__ ("3") = (long)(arg2);		\
	register long _arg3 __asm__ ("4") = (long)(arg3);		\
									\
	__asm__  volatile (						\
		"svc 0\n"						\
		: "+d"(_arg1)						\
		: "d"(_arg2), "d"(_arg3), "d"(_num)			\
		: "memory", "cc"					\
		);							\
	_arg1;								\
})
#elif defined(__loongarch__)
#define SP	"$sp"
#define SP_ALIGNMENT	16
#define SYS_execve	221
#define SYS_exit	93
#define my_syscall3(num, arg1, arg2, arg3)                                    \
({                                                                            \
	register long _num  __asm__ ("a7") = (num);                           \
	register long _arg1 __asm__ ("a0") = (long)(arg1);                    \
	register long _arg2 __asm__ ("a1") = (long)(arg2);                    \
	register long _arg3 __asm__ ("a2") = (long)(arg3);                    \
									      \
	__asm__  volatile (                                                   \
		"syscall 0\n"                                                 \
		: "+r"(_arg1)                                                 \
		: "r"(_arg2), "r"(_arg3),                                     \
		  "r"(_num)                                                   \
		: "memory", "$t0", "$t1", "$t2", "$t3",                       \
		  "$t4", "$t5", "$t6", "$t7", "$t8"                           \
	);                                                                    \
	_arg1;                                                                \
})
#endif

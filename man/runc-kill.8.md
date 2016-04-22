# NAME
   runc kill - kill sends the specified signal (default: SIGTERM) to the container's init process

# SYNOPSIS
   runc kill <container-id> <signal>

Where "<container-id>" is the name for the instance of the container and
"<signal>" is the signal to be sent to the init process.

The mapping between "<signal>" and the real golang signal is:

    "ABRT":   syscall.SIGABRT,
    "ALRM":   syscall.SIGALRM,
    "BUS":    syscall.SIGBUS,
    "CHLD":   syscall.SIGCHLD,
    "CLD":    syscall.SIGCLD,
    "CONT":   syscall.SIGCONT,
    "FPE":    syscall.SIGFPE,
    "HUP":    syscall.SIGHUP,
    "ILL":    syscall.SIGILL,
    "INT":    syscall.SIGINT,
    "IO":     syscall.SIGIO,
    "IOT":    syscall.SIGIOT,
    "KILL":   syscall.SIGKILL,
    "PIPE":   syscall.SIGPIPE,
    "POLL":   syscall.SIGPOLL,
    "PROF":   syscall.SIGPROF,
    "PWR":    syscall.SIGPWR,
    "QUIT":   syscall.SIGQUIT,
    "SEGV":   syscall.SIGSEGV,
    "STKFLT": syscall.SIGSTKFLT,
    "STOP":   syscall.SIGSTOP,
    "SYS":    syscall.SIGSYS,
    "TERM":   syscall.SIGTERM,
    "TRAP":   syscall.SIGTRAP,
    "TSTP":   syscall.SIGTSTP,
    "TTIN":   syscall.SIGTTIN,
    "TTOU":   syscall.SIGTTOU,
    "UNUSED": syscall.SIGUNUSED,
    "URG":    syscall.SIGURG,
    "USR1":   syscall.SIGUSR1,
    "USR2":   syscall.SIGUSR2,
    "VTALRM": syscall.SIGVTALRM,
    "WINCH":  syscall.SIGWINCH,
    "XCPU":   syscall.SIGXCPU,
    "XFSZ":   syscall.SIGXFSZ,


# EXAMPLE

For example, if the container id is "ubuntu01" the following will send a
"syscall.SIGKILL" signal to the init process of the "ubuntu01" container:

    # runc kill ubuntu01 KILL

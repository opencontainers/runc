#/bin/bash

chmod 755 hack/seccomp.pl
hack/seccomp.pl < hack/syscall.sample > seccomp/seccompsyscall.go

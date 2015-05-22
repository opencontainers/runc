#/bin/bash
cat seccomp/seccomp_main.go | sed '1,5d' > ~/seccomp_main.go 
hack/seccomp.pl < hack/syscall.sample > seccomp/seccompsyscall.go

package devices

// The code below is copied from
// github.com/cilium/ebpf/internal/sys/signals.go@v0.22.0.

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// A sigset containing only SIGPROF.
var profSet unix.Sigset_t

func init() {
	// See github.com/cilium/ebpf/internal/sys.sigsetAdd for implementation
	// details. Open coded here so that the compiler will check the
	// constant calculations for us.
	profSet.Val[sigprofBit/wordBits] |= 1 << (sigprofBit % wordBits)
}

// maskProfilerSignal locks the calling goroutine to its underlying OS thread
// and adds SIGPROF to the thread's signal mask. This prevents pprof from
// interrupting expensive syscalls like e.g. BPF_PROG_LOAD.
//
// The caller must defer unmaskProfilerSignal() to reverse the operation.
func maskProfilerSignal() {
	runtime.LockOSThread()

	if err := unix.PthreadSigmask(unix.SIG_BLOCK, &profSet, nil); err != nil {
		runtime.UnlockOSThread()
		panic(fmt.Errorf("masking profiler signal: %w", err))
	}
}

// unmaskProfilerSignal removes SIGPROF from the underlying thread's signal
// mask, allowing it to be interrupted for profiling once again.
//
// It also unlocks the current goroutine from its underlying OS thread.
func unmaskProfilerSignal() {
	defer runtime.UnlockOSThread()

	if err := unix.PthreadSigmask(unix.SIG_UNBLOCK, &profSet, nil); err != nil {
		panic(fmt.Errorf("unmasking profiler signal: %w", err))
	}
}

const (
	// Signal is the nth bit in the bitfield.
	sigprofBit = int(unix.SIGPROF - 1)
	// The number of bits in one Sigset_t word.
	wordBits = int(unsafe.Sizeof(unix.Sigset_t{}.Val[0])) * 8
)

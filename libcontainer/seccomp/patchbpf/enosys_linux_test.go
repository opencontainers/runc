//go:build cgo && seccomp
// +build cgo,seccomp

package patchbpf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"

	libseccomp "github.com/seccomp/libseccomp-golang"
	"golang.org/x/net/bpf"
)

type seccompData struct {
	Syscall uint32 // NOTE: We assume sizeof(int) == 4.
	Arch    uint32
	IP      uint64
	Args    [6]uint64
}

// mockSyscallPayload creates a fake seccomp_data struct with the given data.
func mockSyscallPayload(t *testing.T, sysno libseccomp.ScmpSyscall, arch nativeArch, args ...uint64) []byte {
	var buf bytes.Buffer

	data := seccompData{
		Syscall: uint32(sysno),
		Arch:    uint32(arch),
		IP:      0xDEADBEEFCAFE,
	}

	copy(data.Args[:], args)
	if len(args) > 6 {
		t.Fatalf("bad syscall payload: linux only supports 6-argument syscalls")
	}

	// NOTE: We use BigEndian here because golang.org/x/net/bpf assumes that
	//       all payloads are big-endian while seccomp uses host endianness.
	if err := binary.Write(&buf, binary.BigEndian, data); err != nil {
		t.Fatalf("bad syscall payload: cannot write data: %v", err)
	}
	return buf.Bytes()
}

// retFallthrough is returned by the mockFilter. If a the mock filter returns
// this value, it indicates "fallthrough to libseccomp-generated filter".
const retFallthrough uint32 = 0xDEADBEEF

// mockFilter returns a BPF VM that contains a mock filter with an -ENOSYS
// stub. If the filter returns retFallthrough, the stub filter has permitted
// the syscall to pass.
func mockFilter(t *testing.T, config *configs.Seccomp) (*bpf.VM, []bpf.Instruction) {
	patch, err := generatePatch(config)
	if err != nil {
		t.Fatalf("mock filter: generate enosys patch: %v", err)
	}

	program := append(patch, bpf.RetConstant{Val: retFallthrough})

	vm, err := bpf.NewVM(program)
	if err != nil {
		t.Fatalf("mock filter: compile BPF VM: %v", err)
	}
	return vm, program
}

// fakeConfig generates a fake libcontainer seccomp configuration. The syscalls
// are added with an action distinct from the default action.
func fakeConfig(defaultAction configs.Action, explicitSyscalls []string, arches []string) *configs.Seccomp {
	config := configs.Seccomp{
		DefaultAction: defaultAction,
		Architectures: arches,
	}
	syscallAction := configs.Allow
	if syscallAction == defaultAction {
		syscallAction = configs.Kill
	}
	for _, syscall := range explicitSyscalls {
		config.Syscalls = append(config.Syscalls, &configs.Syscall{
			Name:   syscall,
			Action: syscallAction,
		})
	}
	return &config
}

// List copied from <libcontainer/seccomp/config.go>.
var testArches = []string{
	"x86",
	"amd64",
	"x32",
	"arm",
	"arm64",
	"mips",
	"mips64",
	"mips64n32",
	"mipsel",
	"mipsel64",
	"mipsel64n32",
	"ppc",
	"ppc64",
	"ppc64le",
	"s390",
	"s390x",
}

func testEnosysStub(t *testing.T, defaultAction configs.Action, arches []string) {
	explicitSyscalls := []string{
		"setns",
		"kcmp",
		"renameat2",
		"copy_file_range",
	}

	implicitSyscalls := []string{
		"clone",
		"openat",
		"read",
		"write",
	}

	futureSyscalls := []libseccomp.ScmpSyscall{1000, 7331}

	// Quick lookups for which arches are enabled.
	archSet := map[string]bool{}
	for _, arch := range arches {
		archSet[arch] = true
	}

	for _, test := range []struct {
		start, end int
	}{
		{0, 1}, // [setns]
		{0, 2}, // [setns, process_vm_readv]
		{1, 2}, // [process_vm_readv]
		{1, 3}, // [process_vm_readv, renameat2, copy_file_range]
		{1, 4}, // [process_vm_readv, renameat2, copy_file_range]
		{3, 4}, // [copy_file_range]
	} {
		allowedSyscalls := explicitSyscalls[test.start:test.end]
		config := fakeConfig(defaultAction, allowedSyscalls, arches)
		filter, program := mockFilter(t, config)

		// The syscalls are in increasing order of newness, so all syscalls
		// after the last allowed syscall will give -ENOSYS.
		enosysStart := test.end

		for _, arch := range testArches {
			type syscallTest struct {
				syscall  string
				sysno    libseccomp.ScmpSyscall
				expected uint32
			}

			scmpArch, err := libseccomp.GetArchFromString(arch)
			if err != nil {
				t.Fatalf("unknown libseccomp architecture %q: %v", arch, err)
			}

			nativeArch, err := archToNative(scmpArch)
			if err != nil {
				t.Fatalf("unknown audit architecture %q: %v", arch, err)
			}

			var syscallTests []syscallTest

			// Add explicit syscalls (whether they will return -ENOSYS
			// depends on the filter rules).
			for idx, syscall := range explicitSyscalls {
				expected := retFallthrough
				if idx >= enosysStart {
					expected = retErrnoEnosys
				}
				sysno, err := libseccomp.GetSyscallFromNameByArch(syscall, scmpArch)
				if err != nil {
					t.Fatalf("unknown syscall %q on arch %q: %v", syscall, arch, err)
				}
				syscallTests = append(syscallTests, syscallTest{
					syscall,
					sysno,
					expected,
				})
			}

			// Add implicit syscalls.
			for _, syscall := range implicitSyscalls {
				sysno, err := libseccomp.GetSyscallFromNameByArch(syscall, scmpArch)
				if err != nil {
					t.Fatalf("unknown syscall %q on arch %q: %v", syscall, arch, err)
				}
				syscallTests = append(syscallTests, syscallTest{
					sysno:    sysno,
					syscall:  syscall,
					expected: retFallthrough,
				})
			}

			// Add future syscalls.
			for _, sysno := range futureSyscalls {
				baseSysno, err := libseccomp.GetSyscallFromNameByArch("copy_file_range", scmpArch)
				if err != nil {
					t.Fatalf("unknown syscall 'copy_file_range' on arch %q: %v", arch, err)
				}
				sysno += baseSysno

				syscallTests = append(syscallTests, syscallTest{
					sysno:    sysno,
					syscall:  fmt.Sprintf("syscall_%#x", sysno),
					expected: retErrnoEnosys,
				})
			}

			// If we're on s390(x) make sure you get -ENOSYS for the "setup"
			// syscall (this is done to work around an issue with s390x's
			// syscall multiplexing which results in unknown syscalls being a
			// setup(2) invocation).
			switch scmpArch {
			case libseccomp.ArchS390, libseccomp.ArchS390X:
				syscallTests = append(syscallTests, syscallTest{
					sysno:    s390xMultiplexSyscall,
					syscall:  "setup",
					expected: retErrnoEnosys,
				})
			}

			// Test syscalls in the explicit list.
			for _, test := range syscallTests {
				// Override the expected value in the two special cases.
				if !archSet[arch] || isAllowAction(defaultAction) {
					test.expected = retFallthrough
				}

				payload := mockSyscallPayload(t, test.sysno, nativeArch, 0x1337, 0xF00BA5)
				// NOTE: golang.org/x/net/bpf returns int here rather
				// than uint32.
				rawRet, err := filter.Run(payload)
				if err != nil {
					t.Fatalf("error running filter: %v", err)
				}
				ret := uint32(rawRet)
				if ret != test.expected {
					t.Logf("mock filter for %v %v:", arches, allowedSyscalls)
					for idx, insn := range program {
						t.Logf("  [%4.1d] %s", idx, insn)
					}
					t.Logf("payload: %#v", payload)
					t.Errorf("filter %s(%d) %q(%d): got %#x, want %#x", arch, nativeArch, test.syscall, test.sysno, ret, test.expected)
				}
			}
		}
	}
}

var testActions = map[string]configs.Action{
	"allow": configs.Allow,
	"log":   configs.Log,
	"errno": configs.Errno,
	"kill":  configs.Kill,
}

func TestEnosysStub_SingleArch(t *testing.T) {
	for _, arch := range testArches {
		arches := []string{arch}
		t.Run("arch="+arch, func(t *testing.T) {
			for name, action := range testActions {
				t.Run("action="+name, func(t *testing.T) {
					testEnosysStub(t, action, arches)
				})
			}
		})
	}
}

func TestEnosysStub_MultiArch(t *testing.T) {
	for end := 0; end < len(testArches); end++ {
		for start := 0; start < end; start++ {
			arches := testArches[start:end]
			if len(arches) <= 1 {
				continue
			}
			for _, action := range testActions {
				testEnosysStub(t, action, arches)
			}
		}
	}
}

func TestDisassembleHugeFilterDoesNotHang(t *testing.T) {
	hugeFilter, err := libseccomp.NewFilter(libseccomp.ActAllow)
	if err != nil {
		t.Fatalf("failed to create seccomp filter: %v", err)
	}

	for i := 1; i < 10000; i++ {
		if err := hugeFilter.AddRule(libseccomp.ScmpSyscall(i), libseccomp.ActKillThread); err != nil {
			t.Fatalf("failed to add rule to filter %d: %v", i, err)
		}
	}

	_, err = disassembleFilter(hugeFilter)
	if err != nil {
		t.Fatalf("failed to disassembleFilter: %v", err)
	}

	// if we exit, we did not hang
}

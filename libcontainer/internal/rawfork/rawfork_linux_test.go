package rawfork

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func run(t *testing.T, p *Program) error {
	t.Helper()
	proc, err := p.Start()
	if err != nil {
		t.Fatal(err)
	}
	return proc.Wait()
}

func TestEmpty(t *testing.T) {
	if err := run(t, &Program{}); err != nil {
		t.Fatal(err)
	}
}

func TestWrite(t *testing.T) {
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	defer wr.Close()

	const msg = "hello from the child"
	var p Program
	buf := []byte(msg)
	p.Ops = []Op{
		{Name: "write", Trap: unix.SYS_WRITE, Args: [6]uintptr{wr.Fd(), p.Bytes(buf), uintptr(len(buf))}},
	}
	if err := run(t, &p); err != nil {
		t.Fatal(err)
	}
	wr.Close()
	got, err := io.ReadAll(rd)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != msg {
		t.Fatalf("want %q, got %q", msg, got)
	}
}

func TestOpError(t *testing.T) {
	var p Program
	p.Ops = []Op{
		{Name: "getpid", Trap: unix.SYS_GETPID},
		{Name: "close bad fd", Trap: unix.SYS_CLOSE, Args: [6]uintptr{^uintptr(0)}},
		{Name: "not reached", Trap: unix.SYS_GETPID},
	}
	err := run(t, &p)
	var serr *os.SyscallError
	if !errors.As(err, &serr) || serr.Syscall != "close bad fd" || !errors.Is(err, unix.EBADF) {
		t.Fatalf("want close EBADF error, got %v", err)
	}
}

func TestExitStatus(t *testing.T) {
	var p Program
	p.Ops = []Op{
		{Name: "exit", Trap: unix.SYS_EXIT_GROUP, Args: [6]uintptr{42}},
	}
	err := run(t, &p)
	if err == nil || !strings.Contains(err.Error(), "status 42") {
		t.Fatalf("want exit status 42 error, got %v", err)
	}
}

// TestCat checks registers and jumps by implementing cat(1).
func TestCat(t *testing.T) {
	want := bytes.Repeat([]byte("0123456789abcdef"), 1000)
	name := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(name, want, 0o600); err != nil {
		t.Fatal(err)
	}
	rd, wr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer rd.Close()
	defer wr.Close()

	var p Program
	path, err := p.String(name)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 7) // Small and odd, to test the loop.
	fdcwd := unix.AT_FDCWD
	p.Ops = []Op{
		{Name: "open", Trap: unix.SYS_OPENAT, Args: [6]uintptr{uintptr(fdcwd), path, unix.O_RDONLY}, Ret: R3},
		{Name: "read", Trap: unix.SYS_READ, Args: [6]uintptr{0, p.Bytes(buf), uintptr(len(buf))}, ArgRegs: [6]Reg{R3}, Ret: R1, Cond: IfZero, Goto: 3},
		{Name: "write", Trap: unix.SYS_WRITE, Args: [6]uintptr{wr.Fd(), p.Bytes(buf)}, ArgRegs: [6]Reg{2: R1}, Cond: Always, Goto: 1},
	}
	proc, err := p.Start()
	if err != nil {
		t.Fatal(err)
	}
	wr.Close()
	got, readErr := io.ReadAll(rd)
	if err := proc.Wait(); err != nil {
		t.Fatal(err)
	}
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("want %d bytes, got %d bytes: %q", len(want), len(got), got)
	}
}

func TestValidate(t *testing.T) {
	for _, op := range []Op{
		{Name: "bad arg reg", ArgRegs: [6]Reg{5: numRegs + 1}},
		{Name: "bad ret reg", Ret: numRegs + 1},
		{Name: "bad goto", Cond: Always, Goto: 2},
		{Name: "negative goto", Cond: IfZero, Goto: -1},
		{Name: "bad cond", Cond: IfZero + 1},
	} {
		p := Program{Ops: []Op{op}}
		if _, err := p.Start(); err == nil || !strings.Contains(err.Error(), op.Name) {
			t.Errorf("%s: want validation error, got %v", op.Name, err)
		}
	}
}

// TestChildCalls checks that the code run in the child only calls the allowed
// functions. In particular, this catches bounds check panics and calls to
// non-nosplit functions, which the compiler and linker are fine with.
func TestChildCalls(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("only amd64 is supported")
	}
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	// The binary built by "go test" has no symbol table, so build our own.
	bin := filepath.Join(t.TempDir(), "rawfork.test")
	cmd := exec.Command(gobin, "test", "-c", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", cmd, err, out)
	}

	const pkg = "github.com/opencontainers/runc/libcontainer/internal/rawfork."
	cmd = exec.Command(gobin, "tool", "objdump", "-s", "^"+regexp.QuoteMeta(pkg), bin)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", cmd, err)
	}
	// Map of package functions to lines with call instructions.
	calls := map[string][]string{}
	callRe := regexp.MustCompile(`\tCALL (\S+)`)
	var fn string
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		if f, ok := strings.CutPrefix(line, "TEXT "); ok {
			fn, _, _ = strings.Cut(f, "(SB)")
			calls[fn] = nil
			continue
		}
		if callRe.MatchString(line) {
			calls[fn] = append(calls[fn], line)
		}
	}

	allowed := map[string]bool{
		"syscall.RawSyscall(SB)":                      true,
		"syscall.RawSyscall6(SB)":                     true,
		"internal/runtime/syscall/linux.Syscall6(SB)": true,
	}
	seen := map[string]bool{}
	var check func(fn string)
	check = func(fn string) {
		if seen[fn] {
			return
		}
		seen[fn] = true
		lines, ok := calls[fn]
		if !ok {
			t.Errorf("function %s not found", fn)
			return
		}
		for _, line := range lines {
			callee := callRe.FindStringSubmatch(line)[1]
			if f, ok := strings.CutSuffix(callee, "(SB)"); ok && strings.HasPrefix(f, pkg) {
				check(f)
			} else if !allowed[callee] {
				t.Errorf("%s: disallowed call: %s", fn, strings.TrimSpace(line))
			}
		}
	}
	check(pkg + "runChild")
}

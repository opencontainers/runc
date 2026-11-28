package rawfork

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Reg is a child register, which can hold a system call result to be used as
// an argument of a subsequent system call.
type Reg uint8

// Child registers. NoReg means that no register is used.
const (
	NoReg Reg = iota
	R1
	R2
	R3
	R4
)

// numRegs is the number of registers. Must be a power of 2.
const numRegs = 4

// Cond is a condition for jumping to [Op.Goto].
type Cond uint8

// Jump conditions (see [Op.Cond]).
const (
	Never  Cond = iota // Never jump, continue with the next op.
	Always             // Always jump.
	IfZero             // Jump if the system call returned 0.
)

// Op is a single operation (a system call) performed by the child.
//
// If the system call fails, the child exits, and [Process.Wait] returns an
// [os.SyscallError] with the Name of the failed op.
type Op struct {
	// Name describes the op in error messages.
	Name string
	// Trap is the system call number.
	Trap uintptr
	// Args are the system call arguments. Pointers must be obtained using
	// [Program.Pin] or one of its wrappers.
	Args [6]uintptr
	// ArgRegs, if ArgRegs[i] is not NoReg, makes the child use the value of
	// the specified register instead of Args[i].
	ArgRegs [6]Reg
	// Ret, if not NoReg, is the register to store the system call result in.
	Ret Reg
	// Cond and Goto set the index of the op to perform next. Goto equal
	// to the number of ops ends the program.
	Cond Cond
	Goto int
}

// Program is a sequence of ops to be performed by the child.
//
// Once the last op is performed, the child exits with the status of 0.
type Program struct {
	Ops []Op

	pins []unsafe.Pointer
}

// Pin returns the address of ptr suitable for use in [Op.Args], and keeps the
// memory it points to alive until the program is started.
func (p *Program) Pin(ptr unsafe.Pointer) uintptr {
	// Storing ptr in p also makes it escape to the heap,
	// so it won't be moved by stack growth.
	p.pins = append(p.pins, ptr)
	return uintptr(ptr)
}

// Bytes is like [Program.Pin] but for a byte slice.
func (p *Program) Bytes(b []byte) uintptr {
	return p.Pin(unsafe.Pointer(unsafe.SliceData(b)))
}

// String is like [Program.Pin] but for a NUL-terminated copy of s.
func (p *Program) String(s string) (uintptr, error) {
	b, err := unix.BytePtrFromString(s)
	if err != nil {
		return 0, err
	}
	return p.Pin(unsafe.Pointer(b)), nil
}

func (p *Program) validate() error {
	for i, op := range p.Ops {
		for _, r := range op.ArgRegs {
			if r > numRegs {
				return fmt.Errorf("op %d (%s): invalid arg register %d", i, op.Name, r)
			}
		}
		if op.Ret > numRegs {
			return fmt.Errorf("op %d (%s): invalid ret register %d", i, op.Name, op.Ret)
		}
		switch op.Cond {
		case Never:
		case Always, IfZero:
			if op.Goto < 0 || op.Goto > len(p.Ops) {
				return fmt.Errorf("op %d (%s): invalid goto %d", i, op.Name, op.Goto)
			}
		default:
			return fmt.Errorf("op %d (%s): invalid cond %d", i, op.Name, op.Cond)
		}
	}
	return nil
}

// Process is a running child started by [Program.Start].
type Process struct {
	Pid int

	ops     []Op
	errPipe *os.File
}

// Start forks a child process which runs p.
//
// The child inherits all file descriptors of the parent, including those
// with O_CLOEXEC flag set. Note that until the child exits, it may keep
// open the descriptors which are closed by the parent.
func (p *Program) Start() (*Process, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	rd, wr, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	pid, errno := forkAndRun(p.Ops, wr.Fd())
	runtime.KeepAlive(p)
	wr.Close()
	if errno != 0 {
		rd.Close()
		return nil, os.NewSyscallError("clone", errno)
	}
	return &Process{Pid: pid, ops: p.Ops, errPipe: rd}, nil
}

// Wait waits for the child to exit. It returns an [os.SyscallError] if an op
// has failed, or a generic error if the child exited abnormally.
func (c *Process) Wait() error {
	var info [8]byte
	// EOF (no error info) is fine.
	n, _ := io.ReadFull(c.errPipe, info[:])
	c.errPipe.Close()

	var (
		ws  unix.WaitStatus
		err error
	)
	for {
		_, err = unix.Wait4(c.Pid, &ws, 0, nil)
		if err != unix.EINTR {
			break
		}
	}
	if err != nil {
		return os.NewSyscallError("wait4", err)
	}

	if n == len(info) {
		idx := int(binary.NativeEndian.Uint32(info[0:]))
		errno := syscall.Errno(binary.NativeEndian.Uint32(info[4:]))
		name := fmt.Sprintf("op %d", idx)
		if idx >= 0 && idx < len(c.ops) {
			name = c.ops[idx].Name
		}
		return os.NewSyscallError(name, errno)
	}
	switch {
	case ws.Exited() && ws.ExitStatus() == 0:
		return nil
	case ws.Exited():
		return fmt.Errorf("child exited with status %d", ws.ExitStatus())
	case ws.Signaled():
		return fmt.Errorf("child killed by signal %v", ws.Signal())
	}
	return errors.New("child exited abnormally")
}

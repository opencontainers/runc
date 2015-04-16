package seccomp

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

type sockFilter struct {
	code uint16
	jt   uint8
	jf   uint8
	k    uint32
}

type sockFprog struct {
	len  uint16
	filt []sockFilter
}

type Action struct {
	syscall uint32
	action  int
	args    []string
}

type ScmpCtx struct {
	CallMap map[string]Action
	act     int
}

var ScmpActAllow = 0

func ScmpInit(action int) (*ScmpCtx, error) {
	ctx := ScmpCtx{
		CallMap: make(map[string]Action),
		act:     action,
	}
	return &ctx, nil
}

func ScmpAdd(ctx *ScmpCtx, call string, action int, args ...string) error {
	_, exists := ctx.CallMap[call]
	if exists {
		return errors.New("syscall exist")
	}

	//fmt.Printf("%s\n", call)

	sysCall, sysExists := SyscallMap[call]
	if sysExists {
		ctx.CallMap[call] = Action{sysCall, action, args}
		return nil
	}
	return errors.New("syscall not surport")
}

func ScmpDel(ctx *ScmpCtx, call string) error {
	_, exists := ctx.CallMap[call]
	if exists {
		delete(ctx.CallMap, call)
		return nil
	}

	return errors.New("syscall not exist")
}

func ScmpBpfStmt(code uint16, k uint32) sockFilter {
	return sockFilter{code, 0, 0, k}
}

func ScmpBpfJump(code uint16, k uint32, jt, jf uint8) sockFilter {
	return sockFilter{code, jt, jf, k}
}

func prctl(option int, arg2, arg3, arg4, arg5 uintptr) (err error) {
	_, _, e1 := syscall.Syscall6(syscall.SYS_PRCTL, uintptr(option), arg2, arg3, arg4, arg5, 0)
	if e1 != 0 {
		err = e1
	}
	return nil
}

func scmpfilter(prog *sockFprog) (err error) {
	_, _, e1 := syscall.Syscall(syscall.SYS_PRCTL, uintptr(syscall.PR_SET_SECCOMP),
		uintptr(SECCOMP_MODE_FILTER), uintptr(unsafe.Pointer(prog)))
	if e1 != 0 {
		err = e1
	}
	return nil
}

func ScmpLoad(ctx *ScmpCtx) error {
	for key := range SyscallMapMin {
		ScmpAdd(ctx, key, ScmpActAllow)
	}

	num := len(ctx.CallMap)
	filter := make([]sockFilter, num*2+3)

	i := 0
	filter[i] = ScmpBpfStmt(syscall.BPF_LD+syscall.BPF_W+syscall.BPF_ABS, 0)
	i++

	for _, value := range ctx.CallMap {
		filter[i] = ScmpBpfJump(syscall.BPF_JMP+syscall.BPF_JEQ+syscall.BPF_K, value.syscall, 0, 1)
		i++
		filter[i] = ScmpBpfStmt(syscall.BPF_RET+syscall.BPF_K, SECCOMP_RET_ALLOW)
		i++
	}

	filter[i] = ScmpBpfStmt(syscall.BPF_RET+syscall.BPF_K, SECCOMP_RET_TRAP)
	i++
	filter[i] = ScmpBpfStmt(syscall.BPF_RET+syscall.BPF_K, SECCOMP_RET_KILL)
	i++

	prog := sockFprog{
		len:  uint16(i),
		filt: filter,
	}

	if nil != prctl(PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0) {
		fmt.Println("prctl PR_SET_NO_NEW_PRIVS error")
		return errors.New("prctl PR_SET_NO_NEW_PRIVS error")
	}

	if nil != scmpfilter(&prog) {
		fmt.Println("scmpfilter error")
		return errors.New("scmpfilter error")
	}
	return nil
}

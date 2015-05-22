package seccomp

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	EQ = 0
	NE = 1
	GE = 2
	LE = 3
)

const (
	ALLOW = 0
	DENY  = 1
	JUMP  = 2
)

const (
	JUMP_JT  = 0xff
	JUMP_JF  = 0xff
	LABEL_JT = 0xfe
	LABEL_JF = 0xfe
)

const (
	pseudoCall = 30
)

const (
	ScmpActAllow = 0x0

	PF_LD    = 0x0
	BPF_RET  = syscall.BPF_RET
	BPF_K    = syscall.BPF_K
	BPF_ABS  = syscall.BPF_ABS
	BPF_JMP  = syscall.BPF_JMP
	BPF_JEQ  = syscall.BPF_JEQ
	BPF_W    = syscall.BPF_W
	BPF_LD   = syscall.BPF_LD
	BPF_JA   = syscall.BPF_JA
	BPF_MEM  = syscall.BPF_MEM
	BPF_ST   = syscall.BPF_ST
	BPF_JGT  = syscall.BPF_JGT
	BPF_JGE  = syscall.BPF_JGE
	BPF_JSET = syscall.BPF_JSET

	SECCOMP_RET_KILL    = 0x00000000
	SECCOMP_RET_TRAP    = 0x00030000
	SECCOMP_RET_ALLOW   = 0x7fff0000
	SECCOMP_MODE_FILTER = 0x2
	PR_SET_NO_NEW_PRIVS = 0x26
)

type seccompData struct {
	nr         int32
	arch       uint32
	insPointer uint64
	args       [6]uint64
}

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

type FilterArgs struct {
	Args []Filter
}

type Action struct {
	action int
	args   []FilterArgs
}

type Filter struct {
	Arg uint32 //index of args which start from zero
	Op  int    //operation, such ass EQ/NE/GE/LE
	V   uint   //the value of arg
}

type bpfLabel struct {
	label    string
	location uint32
}

type bpfLabels struct {
	count  uint32
	labels []bpfLabel
}

type ScmpCtx struct {
	CallMap map[int]*Action
	filter  []sockFilter
	label   bpfLabels
}

type argOFunc func(uint32) uint32
type argFunc func(*ScmpCtx, uint32)
type jFunc func(*ScmpCtx, uint, sockFilter)
type addFunc func(ctx *ScmpCtx, call int, action int, args ...FilterArgs) error

var secData seccompData = seccompData{0, 0, 0, [6]uint64{0, 0, 0, 0, 0, 0}}
var hiArg argOFunc
var loArg argOFunc
var arg argFunc
var jEq jFunc
var jNe jFunc
var jGe jFunc
var jLe jFunc
var secAdd addFunc = nil

var op [4]jFunc

var (
	sysCallMin = 0
	sysCallMax = 0
)
var sigSec bool = false

func arg32(ctx *ScmpCtx, idx uint32) {
	ctx.filter = append(ctx.filter,
		scmpBpfStmt(BPF_LD+BPF_W+BPF_ABS, loArg(idx)))
}

func jEq32(ctx *ScmpCtx, v uint, jt sockFilter) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, uint32(v), 0, 1))
	ctx.filter = append(ctx.filter, jt)
}

func jNe32(ctx *ScmpCtx, v uint, jt sockFilter) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, uint32(v), 1, 0))
	ctx.filter = append(ctx.filter, jt)
}

func jGe32(ctx *ScmpCtx, v uint, jt sockFilter) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGE+BPF_K, uint32(v), 0, 1))
	ctx.filter = append(ctx.filter, jt)
}

func jLe32(ctx *ScmpCtx, v uint, jt sockFilter) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGT+BPF_K, uint32(v), 1, 0))
	ctx.filter = append(ctx.filter, jt)
}

func arg64(ctx *ScmpCtx, idx uint32) {
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_W+BPF_ABS, loArg(idx)))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_ST, 0))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_W+BPF_ABS, hiArg(idx)))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_ST, 1))
}

func jNe64(ctx *ScmpCtx, v uint, jt sockFilter) {
	lo := uint32(uint64(v) % 0x100000000)
	hi := uint32(uint64(v) / 0x100000000)
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (hi), 5, 0))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (lo), 2, 0))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
	ctx.filter = append(ctx.filter, jt)
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
}

func jGe64(ctx *ScmpCtx, v uint, jt sockFilter) {
	lo := uint32(uint64(v) % 0x100000000)
	hi := uint32(uint64(v) / 0x100000000)
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGT+BPF_K, (hi), 4, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (hi), 0, 5))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGE+BPF_K, (lo), 0, 2))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
	ctx.filter = append(ctx.filter, jt)
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
}

func jEq64(ctx *ScmpCtx, v uint, jt sockFilter) {
	lo := uint32(uint64(v) % 0x100000000)
	hi := uint32(uint64(v) / 0x100000000)
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (hi), 0, 5))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (lo), 0, 2))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
	ctx.filter = append(ctx.filter, jt)
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
}

func jLe64(ctx *ScmpCtx, v uint, jt sockFilter) {
	lo := uint32(uint64(v) % 0x100000000)
	hi := uint32(uint64(v) / 0x100000000)
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGT+BPF_K, (hi), 6, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, (hi), 0, 3))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 0))
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JGT+BPF_K, (lo), 2, 0))
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
	ctx.filter = append(ctx.filter, jt)
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_LD+BPF_MEM, 1))
}

func allow(ctx *ScmpCtx) {
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_RET+BPF_K, SECCOMP_RET_ALLOW))
}

func deny(ctx *ScmpCtx) {
	ctx.filter = append(ctx.filter, scmpBpfStmt(BPF_RET+BPF_K, SECCOMP_RET_TRAP))
}

func jump(ctx *ScmpCtx, lb string) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JA, findLabel(&ctx.label, lb),
		JUMP_JT, JUMP_JF))
}

func label(ctx *ScmpCtx, lb string) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JA, findLabel(&ctx.label, lb),
		LABEL_JT, LABEL_JF))
}

func secCall(ctx *ScmpCtx, nr int, jt sockFilter) {
	ctx.filter = append(ctx.filter, scmpBpfJump(BPF_JMP+BPF_JEQ+BPF_K, uint32(nr), 0, 1))
	ctx.filter = append(ctx.filter, jt)
}

func findLabel(labels *bpfLabels, lb string) uint32 {
	var id uint32
	for id = 0; id < labels.count; id++ {
		if true == strings.EqualFold(lb, labels.labels[id].label) {
			return id
		}
	}
	tlabel := bpfLabel{lb, 0xffffffff}
	labels.labels = append(labels.labels, tlabel)
	labels.count += 1
	return id
}

func hiArgLittle(idx uint32) uint32 {
	if idx < 0 || idx >= 6 {
		return 0
	}

	hi := uint32(unsafe.Offsetof(secData.args)) + uint32(unsafe.Alignof(secData.args[0]))*idx + uint32(unsafe.Sizeof(secData.arch))
	return uint32(hi)
}

func hiArgBig(idx uint32) uint32 {
	if idx >= 6 {
		return 0
	}
	hi := uint32(unsafe.Offsetof(secData.args)) + 8*idx
	return uint32(hi)
}

func isLittle() bool {
	litEndian := true
	x := 0x1234
	p := unsafe.Pointer(&x)
	p2 := (*[unsafe.Sizeof(0)]byte)(p)
	if p2[0] == 0 {
		litEndian = false
	}
	return litEndian
}

func scmpBpfStmt(code uint16, k uint32) sockFilter {
	return sockFilter{code, 0, 0, k}
}

func scmpBpfJump(code uint16, k uint32, jt, jf uint8) sockFilter {
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

func CombineArgs(args1 []FilterArgs, args2 []FilterArgs) []FilterArgs {
	ilen1 := len(args1)
	if ilen1 > len(args2) {
		ilen1 = len(args2)
	}
	for i1 := 0; i1 < ilen1; i1++ {
		jlen1 := len(args1[i1].Args)
		jlen2 := len(args2[i1].Args)
		for j2 := 0; j2 < jlen2; j2++ {
			num := 0
			for j1 := 0; j1 < jlen1; j1++ {
				if args1[i1].Args[j1] == args2[i1].Args[j2] {
					break
				}
				num = num + 1
			}
			if num == jlen1 {
				args1[i1].Args = append(args1[i1].Args, args2[i1].Args[j2])
			}
		}
	}
	if ilen1 < len(args2) {
		args1 = append(args1, args2[ilen1:]...)
	}
	return args1
}

func Sys(call string) int {
	number, exists := syscallMap[call]
	if exists {
		return number
	}
	return -1
}

func ScmpInit(action int) (*ScmpCtx, error) {
	ctx := ScmpCtx{
		CallMap: make(map[int]*Action),
		filter:  make([]sockFilter, 0, 128),
		label: bpfLabels{
			count:  0,
			labels: make([]bpfLabel, 0, 128),
		},
	}

	ctx.filter = append(ctx.filter,
		sockFilter{PF_LD + BPF_W + BPF_ABS, 0, 0, uint32(unsafe.Offsetof(secData.nr))})
	return &ctx, nil
}

func ScmpDel(ctx *ScmpCtx, call int) error {
	_, exists := ctx.CallMap[call]
	if exists {
		delete(ctx.CallMap, call)
		return nil
	}

	return errors.New("syscall not exist")
}

func ScmpAdd(ctx *ScmpCtx, call int, action int, args ...FilterArgs) error {
	if call < 0 {
		return errors.New("syscall error, call < 0")
	}

	if call <= sysCallMax {
		_, exists := ctx.CallMap[call]
		if exists {
			return errors.New("syscall exist")
		}
		ctx.CallMap[call] = &Action{action, args}
		return nil
	} else {
		if nil != secAdd {
			return secAdd(ctx, call, action, args...)
		}
	}

	return errors.New("syscall not surport")
}

func ScmpLoad(ctx *ScmpCtx) error {
	for call, act := range ctx.CallMap {
		if len(act.args) == 0 {
			secCall(ctx, call, scmpBpfStmt(BPF_RET+BPF_K, SECCOMP_RET_ALLOW))
		} else {
			if len(act.args[0].Args) > 0 {
				lb := fmt.Sprintf("lb-%d-%d", call, act.args[0].Args[0].Arg)
				secCall(ctx, call,
					scmpBpfJump(BPF_JMP+BPF_JA, findLabel(&ctx.label, lb),
						JUMP_JT, JUMP_JF))
			}
		}
	}
	deny(ctx)

	for call, act := range ctx.CallMap {
		for i := 0; i < len(act.args); i++ {
			if len(act.args[i].Args) > 0 {
				lb := fmt.Sprintf("lb-%d-%d", call, act.args[i].Args[0].Arg)
				label(ctx, lb)
				arg(ctx, act.args[i].Args[0].Arg)
			}

			for j := 0; j < len(act.args[i].Args); j++ {
				var jf sockFilter
				if len(act.args)-1 > i && len(act.args[i+1].Args) > 0 {
					lbj := fmt.Sprintf("lb-%d-%d", call, act.args[i+1].Args[0].Arg)
					jf = scmpBpfJump(BPF_JMP+BPF_JA,
						findLabel(&ctx.label, lbj), JUMP_JT, JUMP_JF)
				} else {
					jf = scmpBpfStmt(BPF_RET+BPF_K, SECCOMP_RET_ALLOW)
				}
				op[act.args[i].Args[j].Op](ctx, act.args[i].Args[j].V, jf)
			}

			deny(ctx)
		}
	}

	idx := int32(len(ctx.filter) - 1)
	for ; idx >= 0; idx-- {
		filter := &ctx.filter[idx]
		if filter.code != (BPF_JMP + BPF_JA) {
			continue
		}

		rel := int32(filter.jt)<<8 | int32(filter.jf)
		if ((JUMP_JT << 8) | JUMP_JF) == rel {
			if ctx.label.labels[filter.k].location == 0xffffffff {
				return errors.New("Unresolved label")
			}
			filter.k = ctx.label.labels[filter.k].location - uint32(idx+1)
			filter.jt = 0
			filter.jf = 0
		} else if ((LABEL_JT << 8) | LABEL_JF) == rel {
			if ctx.label.labels[filter.k].location != 0xffffffff {
				return errors.New("Duplicate label use")
			}
			ctx.label.labels[filter.k].location = uint32(idx)
			filter.k = 0
			filter.jt = 0
			filter.jf = 0
		}
	}
	prog := sockFprog{
		len:  uint16(len(ctx.filter)),
		filt: ctx.filter,
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

func sigSeccomp() {
	sigSec = true
}

func ScmpError() bool {
	ret := sigSec
	sigSec = false
	return ret
}

func init() {
	if runtime.GOARCH == "386" {
		sysCallMax = 340
	} else if runtime.GOARCH == "amd64" {
		sysCallMax = 302
	} else if runtime.GOARCH == "arm" {
		sysCallMax = 377
	} else if runtime.GOARCH == "arm64" {
		sysCallMax = 281
	} else if runtime.GOARCH == "ppc64" {
		sysCallMax = 354
	} else if runtime.GOARCH == "ppc64le" {
		sysCallMax = 354
	}
	if isLittle() {
		hiArg = hiArgLittle
		loArg = hiArgBig
	} else {
		hiArg = hiArgBig
		loArg = hiArgLittle
	}

	var length int
	if 8 == int(unsafe.Sizeof(length)) {
		arg = arg64
		jEq = jEq64
		jNe = jNe64
		jGe = jGe64
		jLe = jLe64
	} else {
		arg = arg32
		jEq = jEq32
		jNe = jNe32
		jGe = jGe32
		jLe = jLe32
	}
	op[EQ] = jEq
	op[NE] = jNe
	op[GE] = jGe
	op[LE] = jLe
	chSignal := make(chan os.Signal)
	signal.Notify(chSignal, syscall.SIGSYS)
	go sigSeccomp()
}

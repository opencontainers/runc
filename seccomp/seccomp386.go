// +build linux
// +build 386

package seccomp

import (
	"errors"
)

var (
	syscallInterval = 100
	ipcNr           = syscallInterval + 0
	socketcallNr    = syscallInterval + ipcNr
	callipc         = 0
	callsocket      = 0
)

func scmpAdd386(ctx *ScmpCtx, call int, action int, args ...FilterArgs) error {
	var syscallNo int
	pseCall := call - sysCallMax
	if (pseCall >= ipcNr) && (pseCall < ipcNr+syscallInterval) {
		syscallNo, _ = syscallMap["ipc"]
		pseCall = (pseCall - ipcNr) % ipcNr

	} else if (pseCall >= socketcallNr) && (pseCall < socketcallNr+syscallInterval) {
		syscallNo, _ = syscallMap["socketcall"]
		pseCall = (pseCall - socketcallNr) % socketcallNr
	} else {
		return errors.New("scmpAdd386, syscall error")
	}
	act, exists := ctx.CallMap[syscallNo]
	if !exists {
		newArg := make([]FilterArgs, len(args)+1)
		newArg[0].Args = make([]Filter, 1)
		newArg[0].Args[0].Op = EQ
		newArg[0].Args[0].Arg = 0
		newArg[0].Args[0].V = uint(pseCall)
		for i := 0; i < len(args); i++ {
			alen := len(args[i].Args)
			if alen > 0 {
				newArg[i+1].Args = make([]Filter, alen)
				for j := 0; j < alen; i++ {
					newArg[i+1].Args[j].Op = args[i].Args[j].Op
					newArg[i+1].Args[j].Arg = args[i].Args[j].Arg
					newArg[i+1].Args[j].V = args[i].Args[j].V
				}
			}
		}
		ctx.CallMap[syscallNo] = &Action{action, newArg}
	} else {
		newArg := make([]FilterArgs, len(args))
		for i := 0; i < len(args); i++ {
			alen := len(args[i].Args)
			if alen > 0 {
				newArg[i].Args = make([]Filter, alen)
				for j := 0; j < alen; i++ {
					newArg[i].Args[j].Op = args[i].Args[j].Op
					newArg[i].Args[j].Arg = args[i].Args[j].Arg
					newArg[i].Args[j].V = args[i].Args[j].V
				}
			}
		}
		act.args = CombineArgs(act.args, newArg)
	}

	return nil
}

func resetCallipc(call string, num int) {
	syscallMap[call] = num + callipc
}

func resetCallsocket(call string, num int) {
	syscallMap[call] = num + callsocket
}

func init() {
	sysCallMax = 340
	callipc = ipcNr + sysCallMax
	callsocket = socketcallNr + sysCallMax
	secAdd = scmpAdd386

	resetCallipc("semop", 1)
	resetCallipc("semget", 2)
	resetCallipc("semctl", 3)
	resetCallipc("semtimedop", 4)
	resetCallipc("msgsnd", 11)
	resetCallipc("msgrcv", 12)
	resetCallipc("msgget", 13)
	resetCallipc("msgctl", 14)
	resetCallipc("shmat", 21)
	resetCallipc("shmdt", 22)
	resetCallipc("shmget", 23)
	resetCallipc("shmctl", 24)

	resetCallsocket("socket", 1)
	resetCallsocket("bind", 2)
	resetCallsocket("connect", 3)
	resetCallsocket("listen", 4)
	resetCallsocket("accept", 5)
	resetCallsocket("getsockname", 6)
	resetCallsocket("getpeername", 7)
	resetCallsocket("socketpair", 8)
	resetCallsocket("send", 9)
	resetCallsocket("recv", 10)
	resetCallsocket("sendto", 11)
	resetCallsocket("recvfrom", 12)
	resetCallsocket("shutdown", 13)
	resetCallsocket("setsockopt", 14)
	resetCallsocket("getsockopt", 15)
	resetCallsocket("sendmsg", 16)
	resetCallsocket("recvmsg", 17)
	resetCallsocket("accept4", 18)
	resetCallsocket("recvmmsg", 19)
	resetCallsocket("sendmmsg", 20)

}

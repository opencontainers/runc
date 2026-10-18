//go:build cgo && seccomp

package seccomp

import (
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestGetActionErrnoRet(t *testing.T) {
	errnoRet := uint(1)
	tests := []struct {
		name    string
		action  configs.Action
		wantErr bool
	}{
		{name: "kill", action: configs.Kill, wantErr: true},
		{name: "errno", action: configs.Errno},
		{name: "trap", action: configs.Trap, wantErr: true},
		{name: "allow", action: configs.Allow, wantErr: true},
		{name: "trace", action: configs.Trace},
		{name: "log", action: configs.Log, wantErr: true},
		{name: "notify", action: configs.Notify, wantErr: true},
		{name: "kill thread", action: configs.KillThread, wantErr: true},
		{name: "kill process", action: configs.KillProcess, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getAction(tt.action, &errnoRet)
			if tt.wantErr && err == nil {
				t.Fatal("expected errnoRet to be rejected for this action")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestInitSeccompRejectsErrnoRetForAllow(t *testing.T) {
	errnoRet := uint(1)
	tests := []struct {
		name   string
		config *configs.Seccomp
	}{
		{
			name: "default action",
			config: &configs.Seccomp{
				DefaultAction:   configs.Allow,
				DefaultErrnoRet: &errnoRet,
			},
		},
		{
			name: "syscall action",
			config: &configs.Seccomp{
				DefaultAction: configs.Allow,
				Syscalls: []*configs.Syscall{{
					Name:     "getpid",
					Action:   configs.Allow,
					ErrnoRet: &errnoRet,
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := InitSeccomp(tt.config)
			if err == nil {
				t.Fatal("expected errnoRet with SCMP_ACT_ALLOW to be rejected")
			}
			if !strings.Contains(err.Error(), "errnoRet is only supported") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

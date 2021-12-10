package integration

import (
	"os"
	"runtime"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

// Same as ../../init.go but for libcontainer/integration.
func init() {
	if len(os.Args) < 2 || os.Args[1] != "init" {
		return
	}
	// This is the golang entry point for runc init, executed
	// before TestMain() but after libcontainer/nsenter's nsexec().
	runtime.GOMAXPROCS(1)
	runtime.LockOSThread()
	if err := libcontainer.StartInitialization(); err != nil {
		// as the error is sent back to the parent there is no need to log
		// or write it to stderr because the parent process will handle this
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	ret := m.Run()
	os.Exit(ret)
}

package integration

import (
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	//nolint:revive // Enable cgroup manager to manage devices
	_ "github.com/opencontainers/runc/libcontainer/cgroups/devices"
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
		// logrus is not initialized
		fmt.Fprintln(os.Stderr, err)
	}
	// Normally, StartInitialization() never returns, meaning
	// if we are here, it had failed.
	os.Exit(1)
}

func TestMain(m *testing.M) {
	ret := m.Run()
	os.Exit(ret)
}

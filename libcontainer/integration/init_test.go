package integration

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer"
	//nolint:revive // Enable cgroup manager to manage devices
	_ "github.com/opencontainers/runc/libcontainer/cgroups/devices"
	_ "github.com/opencontainers/runc/libcontainer/nsenter"
)

// Same as ../../init.go but for libcontainer/integration.
func init() {
	if len(os.Args) > 1 && os.Args[1] == "init" {
		libcontainer.Init()
	}
}

func TestMain(m *testing.M) {
	ret := m.Run()
	os.Exit(ret)
}

package libcontainer

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"golang.org/x/sys/unix"
)

func TestResetCPUAffinityMaskIncludesMaxCPU(t *testing.T) {
	mask := unix.NewCPUSet(configs.MaxCPU + 1)
	mask.Fill()

	for _, cpu := range []int{0, configs.MaxCPU - 1, configs.MaxCPU} {
		if !mask.IsSet(cpu) {
			t.Errorf("reset mask does not include accepted CPU ID %d", cpu)
		}
	}
	if mask.IsSet(configs.MaxCPU + 1) {
		t.Errorf("reset mask unexpectedly includes CPU ID %d above the accepted maximum", configs.MaxCPU+1)
	}
}

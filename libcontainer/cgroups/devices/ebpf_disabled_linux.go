//go:build runc_no_ebpf

package devices

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func setV2(dirPath string, r *configs.Resources) error {
	return fmt.Errorf("eBPF support is disabled")
}

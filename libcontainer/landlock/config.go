package landlock

import (
	"fmt"

	"github.com/opencontainers/runc/libcontainer/configs"
)

var accessFSs = map[string]configs.AccessFS{
	"execute":     configs.Execute,
	"write_file":  configs.WriteFile,
	"read_file":   configs.ReadFile,
	"read_dir":    configs.ReadDir,
	"remove_dir":  configs.RemoveDir,
	"remove_file": configs.RemoveFile,
	"make_char":   configs.MakeChar,
	"make_dir":    configs.MakeDir,
	"make_reg":    configs.MakeReg,
	"make_sock":   configs.MakeSock,
	"make_fifo":   configs.MakeFifo,
	"make_block":  configs.MakeBlock,
	"make_sym":    configs.MakeSym,
}

// ConvertStringToAccessFS converts a string into a Landlock access right.
func ConvertStringToAccessFS(in string) (configs.AccessFS, error) {
	if access, ok := accessFSs[in]; ok {
		return access, nil
	}
	return 0, fmt.Errorf("string %s is not a valid access right for landlock", in)
}

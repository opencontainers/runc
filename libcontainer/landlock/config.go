package landlock

import (
	"fmt"

	"github.com/landlock-lsm/go-landlock/landlock"
	ll "github.com/landlock-lsm/go-landlock/landlock/syscall"
)

var accessFSSets = map[string]landlock.AccessFSSet{
	"execute":     ll.AccessFSExecute,
	"write_file":  ll.AccessFSWriteFile,
	"read_file":   ll.AccessFSReadFile,
	"read_dir":    ll.AccessFSReadDir,
	"remove_dir":  ll.AccessFSRemoveDir,
	"remove_file": ll.AccessFSRemoveFile,
	"make_char":   ll.AccessFSMakeChar,
	"make_dir":    ll.AccessFSMakeDir,
	"make_reg":    ll.AccessFSMakeReg,
	"make_sock":   ll.AccessFSMakeSock,
	"make_fifo":   ll.AccessFSMakeFifo,
	"make_block":  ll.AccessFSMakeBlock,
	"make_sym":    ll.AccessFSMakeSym,
}

// ConvertStringToAccessFSSet converts a string into a go-landlock AccessFSSet
// access right.
// This gives more explicit control over the mapping between the permitted
// values in the spec and the ones supported in go-landlock library.
func ConvertStringToAccessFSSet(in string) (landlock.AccessFSSet, error) {
	if access, ok := accessFSSets[in]; ok {
		return access, nil
	}
	return 0, fmt.Errorf("string %s is not a valid access right for landlock", in)
}

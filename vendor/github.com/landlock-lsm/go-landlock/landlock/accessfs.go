package landlock

import (
	"fmt"
	"strings"
)

var accessFSNames = []string{
	"execute",
	"write_file",
	"read_file",
	"read_dir",
	"remove_dir",
	"remove_file",
	"make_char",
	"make_dir",
	"make_reg",
	"make_sock",
	"make_fifo",
	"make_block",
	"make_sym",
	"refer",
	"truncate",
	"ioctl_dev",
}

// AccessFSSet is a set of Landlockable file system access operations.
type AccessFSSet uint64

var supportedAccessFS = AccessFSSet((1 << len(accessFSNames)) - 1)

func accessSetString(a uint64, names []string) string {
	if a == 0 {
		return "∅"
	}
	var b strings.Builder
	b.WriteByte('{')
	for i := 0; i < 64; i++ {
		if a&(1<<i) == 0 {
			continue
		}
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		if i < len(names) {
			b.WriteString(names[i])
		} else {
			b.WriteString(fmt.Sprintf("1<<%v", i))
		}
	}
	b.WriteByte('}')
	return b.String()
}

func (a AccessFSSet) String() string {
	return accessSetString(uint64(a), accessFSNames)
}

func (a AccessFSSet) isSubset(b AccessFSSet) bool {
	return a&b == a
}

func (a AccessFSSet) intersect(b AccessFSSet) AccessFSSet {
	return a & b
}

func (a AccessFSSet) union(b AccessFSSet) AccessFSSet {
	return a | b
}

func (a AccessFSSet) isEmpty() bool {
	return a == 0
}

// valid returns true iff the given AccessFSSet is supported by this
// version of go-landlock.
func (a AccessFSSet) valid() bool {
	return a.isSubset(supportedAccessFS)
}

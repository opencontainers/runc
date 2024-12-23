package landlock

import (
	"fmt"
	"strings"
)

var flagNames = []string{
	"Execute",
	"WriteFile",
	"ReadFile",
	"ReadDir",
	"RemoveDir",
	"RemoveFile",
	"MakeChar",
	"MakeDir",
	"MakeReg",
	"MakeSock",
	"MakeFifo",
	"MakeBlock",
	"MakeSym",
}

// AccessFSSet is a set of Landlockable file system access operations.
type AccessFSSet uint64

var supportedAccessFS = AccessFSSet((1 << 13) - 1)

func (a AccessFSSet) String() string {
	if a.isEmpty() {
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
		if i < len(flagNames) {
			b.WriteString(flagNames[i])
		} else {
			b.WriteString(fmt.Sprintf("1<<%v", i))
		}
	}
	b.WriteByte('}')
	return b.String()
}

func (a AccessFSSet) isSubset(b AccessFSSet) bool {
	return a&b == a
}

func (a AccessFSSet) intersect(b AccessFSSet) AccessFSSet {
	return a & b
}

func (a AccessFSSet) isEmpty() bool {
	return a == 0
}

// valid returns true iff the given AccessFSSet is supported by this
// version of go-landlock.
func (a AccessFSSet) valid() bool {
	return a.isSubset(supportedAccessFS)
}

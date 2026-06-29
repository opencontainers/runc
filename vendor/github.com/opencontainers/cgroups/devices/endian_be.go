//go:build armbe || arm64be || mips || mips64 || mips64p32 || ppc64 || s390 || s390x || sparc || sparc64

package devices

import "encoding/binary"

// nativeEndian is used as a workaround for cilium/ebpf/asm
// which does not accept binary.NativeEndian.
var nativeEndian = binary.BigEndian

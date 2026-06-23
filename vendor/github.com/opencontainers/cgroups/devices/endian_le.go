//go:build 386 || amd64 || amd64p32 || arm || arm64 || loong64 || mipsle || mips64le || mips64p32le || ppc64le || riscv64 || wasm

package devices

import "encoding/binary"

// nativeEndian is used as a workaround for cilium/ebpf/asm
// which does not accept binary.NativeEndian.
var nativeEndian = binary.LittleEndian

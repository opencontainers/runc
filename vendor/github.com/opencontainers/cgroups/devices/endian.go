package devices

import (
	"encoding/binary"
)

// nativeEndian is used as a workaround for cilium/ebpf/asm,
// which does not accept binary.NativeEndian.
var nativeEndian binary.ByteOrder = func() binary.ByteOrder {
	buf := [2]byte{}
	binary.NativeEndian.PutUint16(buf[:], 0xABCD)
	switch buf {
	case [2]byte{0xCD, 0xAB}:
		return binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		return binary.BigEndian
	default:
		panic("could not determine native endianness")
	}
}()

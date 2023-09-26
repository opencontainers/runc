//go:build !linux || (!386 && !amd64 && !arm && !arm64 && !loong64 && !ppc64le && !riscv64 && !s390x) || runc_nodmz
// +build !linux !386,!amd64,!arm,!arm64,!loong64,!ppc64le,!riscv64,!s390x runc_nodmz

package dmz

import (
	"os"
)

func Binary(_ string) (*os.File, error) {
	return nil, ErrNoDmzBinary
}

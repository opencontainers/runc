//go:build (386 || amd64 || arm || arm64 || loong64 || ppc64le || riscv64 || s390x) && !runc_nodmz
// +build 386 amd64 arm arm64 loong64 ppc64le riscv64 s390x
// +build !runc_nodmz

package dmz

import (
	"bytes"
	"debug/elf"
	_ "embed"
	"os"

	"github.com/sirupsen/logrus"
)

// Try to build the runc-dmz binary. If it fails, replace it with an empty file
// (this will trigger us to fall back to a clone of /proc/self/exe). Yeah, this
// is a bit ugly but it makes sure that weird cross-compilation setups don't
// break because of runc-dmz.
//
//go:generate sh -c "make -B runc-dmz || echo -n >runc-dmz"
//go:embed runc-dmz
var runcDmzBinary []byte

// Binary returns a cloned copy (see CloneBinary) of a very minimal C program
// that just does an execve() of its arguments. This is used in the final
// execution step of the container execution as an intermediate process before
// the container process is execve'd. This allows for protection against
// CVE-2019-5736 without requiring a complete copy of the runc binary. Each
// call to Binary will return a new copy.
//
// If the runc-dmz binary is not embedded into the runc binary, Binary will
// return ErrNoDmzBinary as the error.
func Binary(tmpDir string) (*os.File, error) {
	rdr := bytes.NewBuffer(runcDmzBinary)
	// Verify that our embedded binary has a standard ELF header.
	if !bytes.HasPrefix(rdr.Bytes(), []byte(elf.ELFMAG)) {
		if rdr.Len() != 0 {
			logrus.Infof("misconfigured build: embedded runc-dmz binary is non-empty but is missing a proper ELF header")
		}
		return nil, ErrNoDmzBinary
	}
	// Setting RUNC_DMZ=legacy disables this dmz method.
	if os.Getenv("RUNC_DMZ") == "legacy" {
		logrus.Debugf("RUNC_DMZ=legacy set -- switching back to classic /proc/self/exe cloning")
		return nil, ErrNoDmzBinary
	}
	return CloneBinary(rdr, int64(rdr.Len()), "runc-dmz", tmpDir)
}

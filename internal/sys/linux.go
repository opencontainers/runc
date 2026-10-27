//go:build linux

package sys

import (
	"fmt"
	"io"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ParentDeathSignal is a parent death signal, as set by PR_SET_PDEATHSIG.
type ParentDeathSignal int

// Restore sets the parent death signal to p, unless p is 0 or the current
// parent death signal is already p.
func (p ParentDeathSignal) Restore() error {
	if p == 0 {
		return nil
	}
	current, err := GetParentDeathSignal()
	if err != nil {
		return err
	}
	if p == current {
		return nil
	}
	return p.Set()
}

// Set sets the parent death signal of the calling thread to p.
func (p ParentDeathSignal) Set() error {
	return SetParentDeathSignal(uintptr(p))
}

// SetParentDeathSignal sets the parent death signal of the calling thread.
func SetParentDeathSignal(sig uintptr) error {
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, sig, 0, 0, 0); err != nil {
		return err
	}
	return nil
}

// GetParentDeathSignal returns the parent death signal of the calling thread.
func GetParentDeathSignal() (ParentDeathSignal, error) {
	var sig int
	if err := unix.Prctl(unix.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0, 0, 0); err != nil {
		return -1, err
	}
	return ParentDeathSignal(sig), nil
}

// Copy is like io.Copy except it uses sendfile(2) if the source and sink are
// both (*os.File) as an optimisation to make copies faster.
func Copy(dst io.Writer, src io.Reader) (copied int64, err error) {
	dstFile, _ := dst.(*os.File)
	srcFile, _ := src.(*os.File)

	if dstFile != nil && srcFile != nil {
		fi, err := srcFile.Stat()
		if err != nil {
			goto fallback
		}
		size := fi.Size()
		for size > 0 {
			n, err := unix.Sendfile(int(dstFile.Fd()), int(srcFile.Fd()), nil, int(size))
			if n > 0 {
				size -= int64(n)
				copied += int64(n)
			}
			if err == unix.EINTR {
				continue
			}
			if err != nil {
				if copied == 0 {
					// If we haven't copied anything so far, we can safely just
					// fallback to io.Copy. We could always do the fallback but
					// it's safer to error out in the case of a partial copy
					// followed by an error (which should never happen).
					goto fallback
				}
				return copied, fmt.Errorf("partial sendfile copy: %w", err)
			}
		}
		return copied, nil
	}

fallback:
	return io.Copy(dst, src)
}

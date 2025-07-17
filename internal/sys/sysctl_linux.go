package sys

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/cyphar/filepath-securejoin/pathrs-lite"
	"github.com/cyphar/filepath-securejoin/pathrs-lite/procfs"
)

func procfsOpenRoot(proc *procfs.Handle, subpath string, flags int) (*os.File, error) {
	handle, err := proc.OpenRoot(subpath)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	return pathrs.Reopen(handle, flags)
}

// WriteSysctls sets the given sysctls to the requested values.
func WriteSysctls(sysctls map[string]string) error {
	// We are going to write multiple sysctls, which require writing to an
	// unmasked procfs which is not going to be cached. To avoid creating a new
	// procfs instance for each one, just allocate one handle for all of them.
	proc, err := procfs.OpenUnsafeProcRoot()
	if err != nil {
		return err
	}
	defer proc.Close()

	for key, value := range sysctls {
		keyPath := strings.ReplaceAll(key, ".", "/")

		sysctlFile, err := procfsOpenRoot(proc, "sys/"+keyPath, unix.O_WRONLY|unix.O_TRUNC|unix.O_CLOEXEC)
		if err != nil {
			return fmt.Errorf("open sysctl %s file: %w", key, err)
		}
		defer sysctlFile.Close()

		n, err := io.WriteString(sysctlFile, value)
		if n != len(value) && err == nil {
			err = fmt.Errorf("short write to file (%d bytes != %d bytes)", n, len(value))
		}
		if err != nil {
			return fmt.Errorf("failed to write sysctl %s = %q: %w", key, value, err)
		}
	}
	return nil
}

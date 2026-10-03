package sys

import (
	"fmt"
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

// sysctlSwapSeparators inverts '.' and '/' in a sysctl key. It is its own
// inverse: applied once to a slash-form key it produces the equivalent
// dot-form key (and vice versa). See sysctlKeyPath for why this is needed.
func sysctlSwapSeparators(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '.':
			return '/'
		case '/':
			return '.'
		}
		return r
	}, s)
}

// sysctlKeyPath converts a sysctl key into the /proc/sys file path it
// refers to. sysctl(8) supports two notations for a key with a literal '.'
// inside one path component (e.g. a VLAN interface name like "eth0.100"):
// the dot form uses '/' for that literal dot ("net.ipv4.conf.eth0/100.rp_filter"),
// and the slash form uses '.' for it ("net/ipv4/conf/eth0.100/rp_filter").
// Naively replacing every '.' with '/' (regardless of which form was given)
// mangles keys that need this escaping, e.g. turning both spellings of the
// eth0.100 example above into ".../conf/eth0/100/rp_filter", which does not
// exist. Normalize to dot form first (a no-op if already in dot form, same
// as the validator's convertSysctlVariableToDotsSeparator), then invert the
// separators once more to get the actual filesystem path.
func sysctlKeyPath(key string) string {
	if firstSep := strings.IndexAny(key, "./"); firstSep != -1 && key[firstSep] == '/' {
		key = sysctlSwapSeparators(key)
	}
	return sysctlSwapSeparators(key)
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
		keyPath := sysctlKeyPath(key)

		sysctlFile, err := procfsOpenRoot(proc, "sys/"+keyPath, unix.O_WRONLY|unix.O_TRUNC|unix.O_CLOEXEC)
		if err != nil {
			return fmt.Errorf("open sysctl %s file: %w", key, err)
		}
		defer sysctlFile.Close()

		_, err = sysctlFile.WriteString(value)
		if err != nil {
			return fmt.Errorf("failed to write sysctl %s = %q: %w", key, value, err)
		}
	}
	return nil
}

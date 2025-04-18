package libcontainer

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestStringifyMountFlags(t *testing.T) {
	for _, test := range []struct {
		name     string
		flags    uintptr
		expected string
	}{
		{"Empty", 0, ""},
		// Single valid flags.
		{"Single-MS_RDONLY", unix.MS_RDONLY, "MS_RDONLY"},
		{"Single-MS_NOSUID", unix.MS_NOSUID, "MS_NOSUID"},
		{"Single-MS_NODEV", unix.MS_NODEV, "MS_NODEV"},
		{"Single-MS_NOEXEC", unix.MS_NOEXEC, "MS_NOEXEC"},
		{"Single-MS_SYNCHRONOUS", unix.MS_SYNCHRONOUS, "MS_SYNCHRONOUS"},
		{"Single-MS_REMOUNT", unix.MS_REMOUNT, "MS_REMOUNT"},
		{"Single-MS_MANDLOCK", unix.MS_MANDLOCK, "MS_MANDLOCK"},
		{"Single-MS_DIRSYNC", unix.MS_DIRSYNC, "MS_DIRSYNC"},
		{"Single-MS_NOSYMFOLLOW", unix.MS_NOSYMFOLLOW, "MS_NOSYMFOLLOW"},
		{"Single-MS_NOATIME", unix.MS_NOATIME, "MS_NOATIME"},
		{"Single-MS_NODIRATIME", unix.MS_NODIRATIME, "MS_NODIRATIME"},
		{"Single-MS_BIND", unix.MS_BIND, "MS_BIND"},
		{"Single-MS_MOVE", unix.MS_MOVE, "MS_MOVE"},
		{"Single-MS_REC", unix.MS_REC, "MS_REC"},
		{"Single-MS_SILENT", unix.MS_SILENT, "MS_SILENT"},
		{"Single-MS_POSIXACL", unix.MS_POSIXACL, "MS_POSIXACL"},
		{"Single-MS_UNBINDABLE", unix.MS_UNBINDABLE, "MS_UNBINDABLE"},
		{"Single-MS_PRIVATE", unix.MS_PRIVATE, "MS_PRIVATE"},
		{"Single-MS_SLAVE", unix.MS_SLAVE, "MS_SLAVE"},
		{"Single-MS_SHARED", unix.MS_SHARED, "MS_SHARED"},
		{"Single-MS_RELATIME", unix.MS_RELATIME, "MS_RELATIME"},
		{"Single-MS_KERNMOUNT", unix.MS_KERNMOUNT, "0x400000"},
		{"Single-MS_I_VERSION", unix.MS_I_VERSION, "MS_I_VERSION"},
		{"Single-MS_STRICTATIME", unix.MS_STRICTATIME, "MS_STRICTATIME"},
		{"Single-MS_LAZYTIME", unix.MS_LAZYTIME, "MS_LAZYTIME"},
		// Invalid flag value.
		{"Unknown-512", 1 << 9, "0x200"},
		// Multiple flag values at the same time.
		{"Multiple-Valid1", unix.MS_RDONLY | unix.MS_REC | unix.MS_BIND, "MS_RDONLY|MS_BIND|MS_REC"},
		{"Multiple-Valid2", unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_REC | unix.MS_NODIRATIME | unix.MS_I_VERSION, "MS_NOSUID|MS_NODEV|MS_NOEXEC|MS_NODIRATIME|MS_REC|MS_I_VERSION"},
		{"Multiple-Mixed", unix.MS_REC | unix.MS_BIND | (1 << 9) | (1 << 31), "MS_BIND|MS_REC|0x80000200"},
	} {
		got := stringifyMountFlags(test.flags)
		if got != test.expected {
			t.Errorf("%s: stringifyMountFlags(0x%x) = %q, expected %q", test.name, test.flags, got, test.expected)
		}
	}
}

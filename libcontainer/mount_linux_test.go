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

func TestMsFlagsToMountAttr(t *testing.T) {
	// allBinaryAttrs is the bitmask of all binary (non-atime) mount attributes.
	allBinaryAttrs := uint64(unix.MOUNT_ATTR_RDONLY | unix.MOUNT_ATTR_NOSUID |
		unix.MOUNT_ATTR_NODEV | unix.MOUNT_ATTR_NOEXEC |
		unix.MOUNT_ATTR_NODIRATIME | unix.MOUNT_ATTR_NOSYMFOLLOW)

	// Helper to compute the expected "cls" when only a subset of binary attrs
	// is set: the set attrs are removed from cls, the rest stay, and the atime
	// field is always cleared.
	expectCls := func(setAttrs uint64) uint64 {
		return (allBinaryAttrs &^ setAttrs) | unix.MOUNT_ATTR__ATIME
	}

	for _, test := range []struct {
		name    string
		flags   int
		wantSet uint64
		wantCls uint64
	}{
		// No flags: all binary attrs cleared, atime defaults to RELATIME.
		{"empty", 0, 0, allBinaryAttrs | unix.MOUNT_ATTR__ATIME},

		// --- Individual binary flags ---
		// Each set flag moves its attr from "cls" to "set"; the rest stay in "cls".
		{
			"rdonly", unix.MS_RDONLY,
			unix.MOUNT_ATTR_RDONLY, expectCls(unix.MOUNT_ATTR_RDONLY),
		},
		{
			"nosuid", unix.MS_NOSUID,
			unix.MOUNT_ATTR_NOSUID, expectCls(unix.MOUNT_ATTR_NOSUID),
		},
		{
			"nodev", unix.MS_NODEV,
			unix.MOUNT_ATTR_NODEV, expectCls(unix.MOUNT_ATTR_NODEV),
		},
		{
			"noexec", unix.MS_NOEXEC,
			unix.MOUNT_ATTR_NOEXEC, expectCls(unix.MOUNT_ATTR_NOEXEC),
		},
		{
			"nodiratime", unix.MS_NODIRATIME,
			unix.MOUNT_ATTR_NODIRATIME, expectCls(unix.MOUNT_ATTR_NODIRATIME),
		},
		{
			"nosymfollow", unix.MS_NOSYMFOLLOW,
			unix.MOUNT_ATTR_NOSYMFOLLOW, expectCls(unix.MOUNT_ATTR_NOSYMFOLLOW),
		},

		// --- Individual atime modes ---
		// MOUNT_ATTR__ATIME is always in "cls" to clear the atime field first.
		{
			"noatime", unix.MS_NOATIME,
			unix.MOUNT_ATTR_NOATIME, expectCls(0),
		},
		{
			"strictatime", unix.MS_STRICTATIME,
			unix.MOUNT_ATTR_STRICTATIME, expectCls(0),
		},
		{
			"relatime", unix.MS_RELATIME,
			0, // MOUNT_ATTR_RELATIME is 0x00, clearing the field is sufficient
			expectCls(0),
		},

		// --- Combinations ---
		{
			"all-binary-flags",
			unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC |
				unix.MS_NODIRATIME | unix.MS_NOSYMFOLLOW,
			allBinaryAttrs, unix.MOUNT_ATTR__ATIME,
		},

		{
			"all-binary-plus-noatime",
			unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC |
				unix.MS_NODIRATIME | unix.MS_NOSYMFOLLOW | unix.MS_NOATIME,
			allBinaryAttrs | unix.MOUNT_ATTR_NOATIME, unix.MOUNT_ATTR__ATIME,
		},

		{
			"all-binary-plus-strictatime",
			unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC |
				unix.MS_NODIRATIME | unix.MS_NOSYMFOLLOW | unix.MS_STRICTATIME,
			allBinaryAttrs | unix.MOUNT_ATTR_STRICTATIME, unix.MOUNT_ATTR__ATIME,
		},

		{
			"all-binary-plus-relatime",
			unix.MS_RDONLY | unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC |
				unix.MS_NODIRATIME | unix.MS_NOSYMFOLLOW | unix.MS_RELATIME,
			allBinaryAttrs, // RELATIME is 0x00, so nothing extra in "set"
			unix.MOUNT_ATTR__ATIME,
		},

		{
			"rdonly-plus-noatime",
			unix.MS_RDONLY | unix.MS_NOATIME,
			unix.MOUNT_ATTR_RDONLY | unix.MOUNT_ATTR_NOATIME,
			expectCls(unix.MOUNT_ATTR_RDONLY),
		},

		// Multiple atime flags set (should not happen in practice, but the
		// switch picks the first: NOATIME > STRICTATIME > RELATIME).
		{
			"noatime-plus-strictatime",
			unix.MS_NOATIME | unix.MS_STRICTATIME,
			unix.MOUNT_ATTR_NOATIME, expectCls(0),
		},
		{
			"strictatime-plus-relatime",
			unix.MS_STRICTATIME | unix.MS_RELATIME,
			unix.MOUNT_ATTR_STRICTATIME, expectCls(0),
		},
	} {
		gotSet, gotCls := msFlagsToMountAttr(test.flags)
		if gotSet != test.wantSet || gotCls != test.wantCls {
			t.Errorf("%s: msFlagsToMountAttr(0x%x) = (set=0x%x, cls=0x%x), want (set=0x%x, cls=0x%x)",
				test.name, test.flags, gotSet, gotCls, test.wantSet, test.wantCls)
		}
	}
}

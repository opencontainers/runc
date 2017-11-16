package mount

import (
	"golang.org/x/sys/unix"
)

func nameToString(name []int8) string {
	buf := make([]byte, 0, len(name))
	for _, i := range name {
		buf = append(buf, byte(i))
	}
	return string(buf)
}

// Get the mount table using the getstatfs syscall
func parseMountTable() ([]*Info, error) {
	n, err := unix.Getfsstat(nil, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	entries := make([]unix.Statfs_t, n)
	_, err = unix.Getfsstat(entries, unix.MNT_NOWAIT)
	if err != nil {
		return nil, err
	}

	var out []*Info
	for _, entry := range entries {
		var mountinfo Info
		mountinfo.Mountpoint = nameToString(entry.Mntonname[:])
		mountinfo.Source = nameToString(entry.Mntfromname[:])
		mountinfo.Fstype = nameToString(entry.Fstypename[:])
		out = append(out, &mountinfo)
	}
	return out, nil
}

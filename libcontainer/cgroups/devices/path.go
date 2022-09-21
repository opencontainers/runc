package devices

import (
	"errors"
	"os"

	"github.com/opencontainers/runc/libcontainer/devices"
	"golang.org/x/sys/unix"
)

var (
	errNotDev   = errors.New("not a block or character device")
	errMismatch = errors.New("device type/major/minor specified do not match those of the actual device")
)

// checkPath checks the Path component of the cgroups device rule, if set. In
// case device type/major/minor are also set, the device is checked to have the
// same type and major:minor. In case those are not set, they are populated
// from the actual device node.
func checkPath(r *devices.Rule) error {
	if r.Path == "" {
		return nil
	}

	var stat unix.Stat_t
	if err := unix.Lstat(r.Path, &stat); err != nil {
		return &os.PathError{Op: "lstat", Path: r.Path, Err: err}
	}

	var (
		devType   devices.Type
		devNumber = uint64(stat.Rdev) //nolint:unconvert // Rdev is uint32 on e.g. MIPS.
		major     = int64(unix.Major(devNumber))
		minor     = int64(unix.Minor(devNumber))
	)
	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFBLK:
		devType = devices.BlockDevice
	case unix.S_IFCHR:
		devType = devices.CharDevice
	default:
		return &os.PathError{Op: "pathCheck", Path: r.Path, Err: errNotDev}
	}

	if r.Minor == -1 && r.Major == -1 && r.Type == devices.WildcardDevice {
		// Those are defaults in specconv.CreateCroupConfig, meaning
		// these fields were not set in spec, so fill them in from the
		// actual device.
		r.Major = major
		r.Minor = minor
		r.Type = devType
		return nil
	}

	// Otherwise (both Path and Type/Major/Minor are specified,
	// which is redundant), do a sanity check.
	if r.Major != major || r.Minor != minor || r.Type != devType {
		return &os.PathError{Op: "pathCheck", Path: r.Path, Err: errMismatch}
	}

	return nil
}

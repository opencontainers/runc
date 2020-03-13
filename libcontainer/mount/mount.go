package mount

// GetMounts retrieves a list of mounts for the current running process,
// with an optional filter applied (use nil for no filter).
func GetMounts(f FilterFunc) ([]*Info, error) {
	return parseMountTable(f)
}

// Mounted determines if a specified mountpoint has been mounted.
// On Linux it looks at /proc/self/mountinfo.
func Mounted(mountpoint string) (bool, error) {
	entries, err := GetMounts(SingleEntryFilter(mountpoint))
	if err != nil {
		return false, err
	}

	return len(entries) > 0, nil
}

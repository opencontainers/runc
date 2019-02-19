package configs

import (
	"fmt"
	"os"
)

const (
	Wildcard = -1
)

// TODO Windows: This can be factored out in the future

type Device struct {
	// Device type, block, char, etc.
	Type rune `json:"type"`

	// Path to the device.
	Path string `json:"path"`

	// Major is the device's major number.
	Major int64 `json:"major"`

	// Minor is the device's minor number.
	Minor int64 `json:"minor"`

	// Cgroup permissions format, rwm.
	Permissions string `json:"permissions"`

	// FileMode permission bits for the device.
	FileMode os.FileMode `json:"file_mode"`

	// Uid of the device.
	Uid uint32 `json:"uid"`

	// Gid of the device.
	Gid uint32 `json:"gid"`

	// Write the file to the allowed list
	Allow bool `json:"allow"`
}

func (d *Device) CgroupString() string {
	return fmt.Sprintf("%c %s:%s %s", d.Type, deviceNumberString(d.Major), deviceNumberString(d.Minor), d.Permissions)
}

func (d *Device) SystemdCgroupPath() string {
	sdType := "char"
	if d.Type == 'b' {
		sdType = "block"
	} else if d.Type != 'c' {
		// TODO: Invalid d.Type, do something about it.
		return ""
	}
	// Start looking for wildcards, blocking a whole major.
	if d.Minor == Wildcard {
		return fmt.Sprintf("%s-%s", sdType, deviceNumberString(d.Major))
	}
	// Systemd uses /dev/char/x:y or /dev/block/x:y for devices by major/minor.
	return fmt.Sprintf("/dev/%s/%s:%s", sdType, deviceNumberString(d.Major), deviceNumberString(d.Minor))
}

func (d *Device) Mkdev() int {
	return int((d.Major << 8) | (d.Minor & 0xff) | ((d.Minor & 0xfff00) << 12))
}

// deviceNumberString converts the device number to a string return result.
func deviceNumberString(number int64) string {
	if number == Wildcard {
		return "*"
	}
	return fmt.Sprint(number)
}

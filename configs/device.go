package configs

import (
	"fmt"
	"os"
)

const (
	Wildcard = -1
)

type Device struct {
	Type rune `json:"type,omitempty"`
	// It is fine if this is an empty string in the case that you are using Wildcards
	Path string `json:"path,omitempty"`
	// Use the wildcard constant for wildcards.
	Major int64 `json:"major,omitempty"`
	// Use the wildcard constant for wildcards.
	Minor int64 `json:"minor,omitempty"`
	// Typically just "rwm"
	Permissions string `json:"permissions,omitempty"`
	// The permission bits of the file's mode
	FileMode os.FileMode `json:"file_mode,omitempty"`
	Uid      uint32      `json:"uid,omitempty"`
	Gid      uint32      `json:"gid,omitempty"`
}

func (d *Device) CgroupString() string {
	return fmt.Sprintf("%c %s:%s %s", d.Type, deviceNumberString(d.Major), deviceNumberString(d.Minor), d.Permissions)
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

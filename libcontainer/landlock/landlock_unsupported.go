// +build !linux

package landlock

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"
)

var ErrLandlockNotSupported = errors.New("land: config provided but Landlock not supported")

// InitLandlock does nothing because Landlock is not supported.
func InitSLandlock(config *configs.Landlock) error {
	if config != nil {
		return ErrLandlockNotSupported
	}
	return nil
}

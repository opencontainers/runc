// +build linux

package mount

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/console"
)

func setupPtmx(config *configs.Config) error {
	ptmx := filepath.Join(config.Rootfs, "dev/ptmx")
	if err := os.Remove(ptmx); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.Symlink("pts/ptmx", ptmx); err != nil {
		return fmt.Errorf("symlink dev ptmx %s", err)
	}
	if config.Console != "" {
		uid, err := config.HostUID()
		if err != nil {
			return err
		}
		gid, err := config.HostGID()
		if err != nil {
			return err
		}
		return console.Setup(config.Rootfs, config.Console, config.MountLabel, uid, gid)
	}
	return nil
}

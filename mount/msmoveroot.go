// +build linux

package mount

import "syscall"

func msMoveRoot(rootfs string) error {
	if err := syscall.Mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return err
	}
	if err := syscall.Chroot("."); err != nil {
		return err
	}
	return syscall.Chdir("/")
}

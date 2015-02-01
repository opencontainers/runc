// +build linux

package mount

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/docker/libcontainer/configs"
	"github.com/docker/libcontainer/label"
)

const defaultMountFlags = syscall.MS_NOEXEC | syscall.MS_NOSUID | syscall.MS_NODEV

type mount struct {
	source string
	path   string
	device string
	flags  int
	data   string
}

// InitializeMountNamespace sets up the devices, mount points, and filesystems for use inside a
// new mount namespace.
func InitializeMountNamespace(config *configs.Config) (err error) {
	if err := prepareRoot(config); err != nil {
		return err
	}
	if err := mountSystem(config); err != nil {
		return err
	}
	// apply any user specified mounts within the new mount namespace
	for _, m := range config.Mounts {
		if err := m.Mount(config.RootFs, config.MountLabel); err != nil {
			return err
		}
	}
	if err := createDeviceNodes(config); err != nil {
		return err
	}
	if err := setupPtmx(config); err != nil {
		return err
	}
	// stdin, stdout and stderr could be pointing to /dev/null from parent namespace.
	// Re-open them inside this namespace.
	// FIXME: Need to fix this for user namespaces.
	if 0 == 0 {
		if err := reOpenDevNull(config.RootFs); err != nil {
			return err
		}
	}
	if err := setupDevSymlinks(config.RootFs); err != nil {
		return err
	}
	if err := syscall.Chdir(config.RootFs); err != nil {
		return err
	}
	if config.NoPivotRoot {
		err = msMoveRoot(config.RootFs)
	} else {
		err = pivotRoot(config.RootFs, config.PivotDir)
	}
	if err != nil {
		return err
	}
	if config.ReadonlyFs {
		if err := setReadonly(); err != nil {
			return fmt.Errorf("set readonly %s", err)
		}
	}
	syscall.Umask(0022)
	return nil
}

// mountSystem sets up linux specific system mounts like mqueue, sys, proc, shm, and devpts
// inside the mount namespace
func mountSystem(config *configs.Config) error {
	for _, m := range newSystemMounts(config.RootFs, config.MountLabel, config.RestrictSys) {
		if err := os.MkdirAll(m.path, 0755); err != nil && !os.IsExist(err) {
			return fmt.Errorf("mkdirall %s %s", m.path, err)
		}
		if err := syscall.Mount(m.source, m.path, m.device, uintptr(m.flags), m.data); err != nil {
			return fmt.Errorf("mounting %s into %s %s", m.source, m.path, err)
		}
	}
	return nil
}

func setupDevSymlinks(rootfs string) error {
	var links = [][2]string{
		{"/proc/self/fd", "/dev/fd"},
		{"/proc/self/fd/0", "/dev/stdin"},
		{"/proc/self/fd/1", "/dev/stdout"},
		{"/proc/self/fd/2", "/dev/stderr"},
	}

	// kcore support can be toggled with CONFIG_PROC_KCORE; only create a symlink
	// in /dev if it exists in /proc.
	if _, err := os.Stat("/proc/kcore"); err == nil {
		links = append(links, [2]string{"/proc/kcore", "/dev/kcore"})
	}

	for _, link := range links {
		var (
			src = link[0]
			dst = filepath.Join(rootfs, link[1])
		)

		if err := os.Symlink(src, dst); err != nil && !os.IsExist(err) {
			return fmt.Errorf("symlink %s %s %s", src, dst, err)
		}
	}

	return nil
}

// TODO: this is crappy right now and should be cleaned up with a better way of handling system and
// standard bind mounts allowing them to be more dynamic
func newSystemMounts(rootfs, mountLabel string, sysReadonly bool) []mount {
	systemMounts := []mount{
		{source: "proc", path: filepath.Join(rootfs, "proc"), device: "proc", flags: defaultMountFlags},
		{source: "tmpfs", path: filepath.Join(rootfs, "dev"), device: "tmpfs", flags: syscall.MS_NOSUID | syscall.MS_STRICTATIME, data: label.FormatMountLabel("mode=755", mountLabel)},
		{source: "shm", path: filepath.Join(rootfs, "dev", "shm"), device: "tmpfs", flags: defaultMountFlags, data: label.FormatMountLabel("mode=1777,size=65536k", mountLabel)},
		{source: "mqueue", path: filepath.Join(rootfs, "dev", "mqueue"), device: "mqueue", flags: defaultMountFlags},
		{source: "devpts", path: filepath.Join(rootfs, "dev", "pts"), device: "devpts", flags: syscall.MS_NOSUID | syscall.MS_NOEXEC, data: label.FormatMountLabel("newinstance,ptmxmode=0666,mode=620,gid=5", mountLabel)},
	}

	sysMountFlags := defaultMountFlags
	if sysReadonly {
		sysMountFlags |= syscall.MS_RDONLY
	}

	systemMounts = append(systemMounts, mount{source: "sysfs", path: filepath.Join(rootfs, "sys"), device: "sysfs", flags: sysMountFlags})

	return systemMounts
}

// Is stdin, stdout or stderr were to be pointing to '/dev/null',
// this method will make them point to '/dev/null' from within this namespace.
func reOpenDevNull(rootfs string) error {
	var stat, devNullStat syscall.Stat_t
	file, err := os.Open(filepath.Join(rootfs, "/dev/null"))
	if err != nil {
		return fmt.Errorf("Failed to open /dev/null - %s", err)
	}
	defer file.Close()
	if err = syscall.Fstat(int(file.Fd()), &devNullStat); err != nil {
		return fmt.Errorf("Failed to stat /dev/null - %s", err)
	}
	for fd := 0; fd < 3; fd++ {
		if err = syscall.Fstat(fd, &stat); err != nil {
			return fmt.Errorf("Failed to stat fd %d - %s", fd, err)
		}
		if stat.Rdev == devNullStat.Rdev {
			// Close and re-open the fd.
			if err = syscall.Dup2(int(file.Fd()), fd); err != nil {
				return fmt.Errorf("Failed to dup fd %d to fd %d - %s", file.Fd(), fd, err)
			}
		}
	}
	return nil
}

// Create the device nodes in the container.
func createDeviceNodes(config *configs.Config) error {
	oldMask := syscall.Umask(0000)
	for _, node := range config.DeviceNodes {
		if err := createDeviceNode(config.RootFs, node); err != nil {
			syscall.Umask(oldMask)
			return err
		}
	}
	syscall.Umask(oldMask)
	return nil
}

// Creates the device node in the rootfs of the container.
func createDeviceNode(rootfs string, node *configs.Device) error {
	var (
		dest   = filepath.Join(rootfs, node.Path)
		parent = filepath.Dir(dest)
	)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	fileMode := node.FileMode
	switch node.Type {
	case 'c':
		fileMode |= syscall.S_IFCHR
	case 'b':
		fileMode |= syscall.S_IFBLK
	default:
		return fmt.Errorf("%c is not a valid device type for device %s", node.Type, node.Path)
	}
	if err := syscall.Mknod(dest, uint32(fileMode), node.Mkdev()); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mknod %s %s", node.Path, err)
	}
	if err := syscall.Chown(dest, int(node.Uid), int(node.Gid)); err != nil {
		return fmt.Errorf("chown %s to %d:%d", node.Path, node.Uid, node.Gid)
	}
	return nil
}

func prepareRoot(config *configs.Config) error {
	flag := syscall.MS_PRIVATE | syscall.MS_REC
	if config.NoPivotRoot {
		flag = syscall.MS_SLAVE | syscall.MS_REC
	}
	if err := syscall.Mount("", "/", "", uintptr(flag), ""); err != nil {
		return err
	}
	return syscall.Mount(config.RootFs, config.RootFs, "bind", syscall.MS_BIND|syscall.MS_REC, "")
}

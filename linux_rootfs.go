// +build linux

package libcontainer

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/docker/docker/pkg/symlink"
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

// setupRootfs sets up the devices, mount points, and filesystems for use inside a
// new mount namespace.
func setupRootfs(config *configs.Config) (err error) {
	if err := prepareRoot(config); err != nil {
		return err
	}
	if err := mountSystem(config); err != nil {
		return err
	}
	// apply any user specified mounts within the new mount namespace
	for _, m := range config.Mounts {
		if err := mountUserMount(m, config.Rootfs, config.MountLabel); err != nil {
			return err
		}
	}
	if err := createDevices(config); err != nil {
		return err
	}
	if err := setupPtmx(config); err != nil {
		return err
	}
	// stdin, stdout and stderr could be pointing to /dev/null from parent namespace.
	// Re-open them inside this namespace.
	// FIXME: Need to fix this for user namespaces.
	if !config.Namespaces.Contains(configs.NEWUSER) {
		if err := reOpenDevNull(config.Rootfs); err != nil {
			return err
		}
	}
	if err := setupDevSymlinks(config.Rootfs); err != nil {
		return err
	}
	if err := syscall.Chdir(config.Rootfs); err != nil {
		return err
	}
	if config.NoPivotRoot {
		err = msMoveRoot(config.Rootfs)
	} else {
		err = pivotRoot(config.Rootfs, config.PivotDir)
	}
	if err != nil {
		return err
	}
	if config.Readonlyfs {
		if err := setReadonly(); err != nil {
			return err
		}
	}
	syscall.Umask(0022)
	return nil
}

// mountSystem sets up linux specific system mounts like mqueue, sys, proc, shm, and devpts
// inside the mount namespace
func mountSystem(config *configs.Config) error {
	for _, m := range newSystemMounts(config.Rootfs, config.MountLabel, config.RestrictSys) {
		if err := os.MkdirAll(m.path, 0755); err != nil && !os.IsExist(err) {
			return err
		}
		if err := syscall.Mount(m.source, m.path, m.device, uintptr(m.flags), m.data); err != nil {
			return err
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
func createDevices(config *configs.Config) error {
	oldMask := syscall.Umask(0000)
	for _, node := range config.Devices {
		if err := createDeviceNode(config.Rootfs, node); err != nil {
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
	return syscall.Mount(config.Rootfs, config.Rootfs, "bind", syscall.MS_BIND|syscall.MS_REC, "")
}

func setReadonly() error {
	return syscall.Mount("/", "/", "bind", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC, "")
}

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
		console := newConsoleFromPath(config.Console)
		return console.mount(config.Rootfs, config.MountLabel, uid, gid)
	}
	return nil
}

func pivotRoot(rootfs, pivotBaseDir string) error {
	if pivotBaseDir == "" {
		pivotBaseDir = "/"
	}
	tmpDir := filepath.Join(rootfs, pivotBaseDir)
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return fmt.Errorf("can't create tmp dir %s, error %v", tmpDir, err)
	}
	pivotDir, err := ioutil.TempDir(tmpDir, ".pivot_root")
	if err != nil {
		return fmt.Errorf("can't create pivot_root dir %s, error %v", pivotDir, err)
	}
	if err := syscall.PivotRoot(rootfs, pivotDir); err != nil {
		return fmt.Errorf("pivot_root %s", err)
	}
	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("chdir / %s", err)
	}
	// path to pivot dir now changed, update
	pivotDir = filepath.Join(pivotBaseDir, filepath.Base(pivotDir))
	if err := syscall.Unmount(pivotDir, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_root dir %s", err)
	}
	return os.Remove(pivotDir)
}

func msMoveRoot(rootfs string) error {
	if err := syscall.Mount(rootfs, "/", "", syscall.MS_MOVE, ""); err != nil {
		return err
	}
	if err := syscall.Chroot("."); err != nil {
		return err
	}
	return syscall.Chdir("/")
}

func mountUserMount(m *configs.Mount, rootfs, mountLabel string) error {
	switch m.Type {
	case "bind":
		return bindMount(m, rootfs, mountLabel)
	case "tmpfs":
		return tmpfsMount(m, rootfs, mountLabel)
	default:
		return fmt.Errorf("unsupported mount type %s for %s", m.Type, m.Destination)
	}
}

func bindMount(m *configs.Mount, rootfs, mountLabel string) error {
	var (
		flags = syscall.MS_BIND | syscall.MS_REC
		dest  = filepath.Join(rootfs, m.Destination)
	)
	if !m.Writable {
		flags = flags | syscall.MS_RDONLY
	}
	if m.Slave {
		flags = flags | syscall.MS_SLAVE
	}
	stat, err := os.Stat(m.Source)
	if err != nil {
		return err
	}
	// TODO: (crosbymichael) This does not belong here and should be done a layer above
	dest, err = symlink.FollowSymlinkInScope(dest, rootfs)
	if err != nil {
		return err
	}
	if err := createIfNotExists(dest, stat.IsDir()); err != nil {
		return fmt.Errorf("creating new bind mount target %s", err)
	}
	if err := syscall.Mount(m.Source, dest, "bind", uintptr(flags), ""); err != nil {
		return err
	}
	if !m.Writable {
		if err := syscall.Mount(m.Source, dest, "bind", uintptr(flags|syscall.MS_REMOUNT), ""); err != nil {
			return err
		}
	}
	if m.Relabel != "" {
		if err := label.Relabel(m.Source, mountLabel, m.Relabel); err != nil {
			return err
		}
	}
	if m.Private {
		if err := syscall.Mount("", dest, "none", uintptr(syscall.MS_PRIVATE), ""); err != nil {
			return err
		}
	}
	return nil
}

func tmpfsMount(m *configs.Mount, rootfs, mountLabel string) error {
	var (
		err  error
		l    = label.FormatMountLabel("", mountLabel)
		dest = filepath.Join(rootfs, m.Destination)
	)
	// TODO: (crosbymichael) This does not belong here and should be done a layer above
	if dest, err = symlink.FollowSymlinkInScope(dest, rootfs); err != nil {
		return err
	}
	if err := createIfNotExists(dest, true); err != nil {
		return err
	}
	return syscall.Mount("tmpfs", dest, "tmpfs", uintptr(defaultMountFlags), l)
}

// createIfNotExists creates a file or a directory only if it does not already exist.
func createIfNotExists(path string, isDir bool) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			if isDir {
				return os.MkdirAll(path, 0755)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE, 0755)
			if err != nil {
				return err
			}
			f.Close()
		}
	}
	return nil
}

// remountReadonly will bind over the top of an existing path and ensure that it is read-only.
func remountReadonly(path string) error {
	for i := 0; i < 5; i++ {
		if err := syscall.Mount("", path, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil && !os.IsNotExist(err) {
			switch err {
			case syscall.EINVAL:
				// Probably not a mountpoint, use bind-mount
				if err := syscall.Mount(path, path, "", syscall.MS_BIND, ""); err != nil {
					return err
				}
				return syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_REC|defaultMountFlags, "")
			case syscall.EBUSY:
				time.Sleep(100 * time.Millisecond)
				continue
			default:
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unable to mount %s as readonly max retries reached", path)
}

// maskProckcore bind mounts /dev/null over the top of /proc/kcore inside a container to avoid security
// issues from processes reading memory information.
func maskProckcore() error {
	if err := syscall.Mount("/dev/null", "/proc/kcore", "", syscall.MS_BIND, ""); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to bind-mount /dev/null over /proc/kcore: %s", err)
	}
	return nil
}

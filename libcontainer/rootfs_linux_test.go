package libcontainer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"

	"golang.org/x/sys/unix"
)

func TestCheckMountDestInProc(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc/sys",
			Source:      "/proc/sys",
			Device:      "bind",
			Flags:       unix.MS_BIND,
		},
	}
	dest := "/rootfs/proc/sys"
	err := checkProcMount("/rootfs", dest, m)
	if err == nil {
		t.Fatal("destination inside proc should return an error")
	}
}

func TestCheckProcMountOnProc(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc",
			Source:      "foo",
			Device:      "proc",
		},
	}
	dest := "/rootfs/proc/"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatalf("procfs type mount on /proc should not return an error: %v", err)
	}
}

func TestCheckBindMountOnProc(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc",
			Source:      "/proc/self",
			Device:      "bind",
			Flags:       unix.MS_BIND,
		},
	}
	dest := "/rootfs/proc/"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatalf("bind-mount of procfs on top of /proc should not return an error (for now): %v", err)
	}
}

func TestCheckTrickyMountOnProc(t *testing.T) {
	// Make a non-bind mount that looks like a bit like a bind-mount.
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc",
			Source:      "/proc",
			Device:      "overlay",
			Data:        "lowerdir=/tmp/fakeproc,upperdir=/tmp/fakeproc2,workdir=/tmp/work",
		},
	}
	dest := "/rootfs/proc/"
	err := checkProcMount("/rootfs", dest, m)
	if err == nil {
		t.Fatalf("dodgy overlayfs mount on top of /proc should return an error")
	}
}

func TestCheckTrickyBindMountOnProc(t *testing.T) {
	// Make a bind mount that looks like it might be a procfs mount.
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc",
			Source:      "/sys",
			Device:      "proc",
			Flags:       unix.MS_BIND,
		},
	}
	dest := "/rootfs/proc/"
	err := checkProcMount("/rootfs", dest, m)
	if err == nil {
		t.Fatalf("dodgy bind-mount on top of /proc should return an error")
	}
}

func TestCheckMountDestInSys(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/sys/fs/cgroup",
			Source:      "tmpfs",
			Device:      "tmpfs",
		},
	}
	dest := "/rootfs//sys/fs/cgroup"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatalf("destination inside /sys should not return an error: %v", err)
	}
}

func TestCheckMountDestFalsePositive(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/sysfiles/fs/cgroup",
			Source:      "tmpfs",
			Device:      "tmpfs",
		},
	}
	dest := "/rootfs/sysfiles/fs/cgroup"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckMountDestNsLastPid(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc/sys/kernel/ns_last_pid",
			Source:      "lxcfs",
			Device:      "fuse.lxcfs",
		},
	}
	dest := "/rootfs/proc/sys/kernel/ns_last_pid"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatalf("/proc/sys/kernel/ns_last_pid should not return an error: %v", err)
	}
}

func TestCheckCryptoFipsEnabled(t *testing.T) {
	m := mountEntry{
		Mount: &configs.Mount{
			Destination: "/proc/sys/crypto/fips_enabled",
			Source:      "tmpfs",
			Device:      "tmpfs",
		},
	}
	dest := "/rootfs/proc/sys/crypto/fips_enabled"
	err := checkProcMount("/rootfs", dest, m)
	if err != nil {
		t.Fatalf("/proc/sys/crypto/fips_enabled should not return an error: %v", err)
	}
}

func TestNeedsSetupDev(t *testing.T) {
	config := &configs.Config{
		Mounts: []*configs.Mount{
			{
				Device:      "bind",
				Source:      "/dev",
				Destination: "/dev",
			},
		},
	}
	if needsSetupDev(config) {
		t.Fatal("expected needsSetupDev to be false, got true")
	}
}

func TestNeedsSetupDevStrangeSource(t *testing.T) {
	config := &configs.Config{
		Mounts: []*configs.Mount{
			{
				Device:      "bind",
				Source:      "/devx",
				Destination: "/dev",
			},
		},
	}
	if needsSetupDev(config) {
		t.Fatal("expected needsSetupDev to be false, got true")
	}
}

func TestNeedsSetupDevStrangeDest(t *testing.T) {
	config := &configs.Config{
		Mounts: []*configs.Mount{
			{
				Device:      "bind",
				Source:      "/dev",
				Destination: "/devx",
			},
		},
	}
	if !needsSetupDev(config) {
		t.Fatal("expected needsSetupDev to be true, got false")
	}
}

func TestNeedsSetupDevStrangeSourceDest(t *testing.T) {
	config := &configs.Config{
		Mounts: []*configs.Mount{
			{
				Device:      "bind",
				Source:      "/devx",
				Destination: "/devx",
			},
		},
	}
	if !needsSetupDev(config) {
		t.Fatal("expected needsSetupDev to be true, got false")
	}
}

type recordedMount struct {
	source      string
	srcFileName string
	srcFileType mountSourceType
	target      string
	dstFd       string
	fstype      string
	flags       uintptr
	data        string
}

func recordMounts(calls *[]recordedMount) mountFunc {
	return func(source string, srcFile *mountSource, target, dstFd, fstype string, flags uintptr, data string) error {
		call := recordedMount{
			source: source,
			target: target,
			dstFd:  dstFd,
			fstype: fstype,
			flags:  flags,
			data:   data,
		}
		if srcFile != nil {
			call.srcFileName = srcFile.file.Name()
			call.srcFileType = srcFile.Type
		}
		*calls = append(*calls, call)
		return nil
	}
}

func TestMaskPathsWithSharedDirMask(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(root, "dir2")
	file := filepath.Join(root, "file")
	missing := filepath.Join(root, "missing")
	rootFd, err := os.OpenFile(root, unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_PATH, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFd.Close()
	for _, dir := range []string{dir1, dir2} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(file, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := mountFn
	t.Cleanup(func() { mountFn = old })
	var calls []recordedMount
	mountLabel := "system_u:object_r:container_file_t:s0"
	mountFn = recordMounts(&calls)
	if err := maskPaths(rootFd, []string{
		missing,
		dir1,
		dir2,
		filepath.Join(dir1, "."),
		file,
	}, mountLabel); err != nil {
		t.Fatal(err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected 3 mount calls, got %d: %#v", len(calls), calls)
	}

	if call := calls[0]; call.source != "tmpfs" || call.fstype != "tmpfs" || call.flags != unix.MS_RDONLY ||
		call.target != dir1 || call.dstFd == "" || call.data != `nr_blocks=1,nr_inodes=1,context="system_u:object_r:container_file_t:s0"` {
		t.Fatalf("unexpected shared tmpfs mount call: %#v", call)
	}
	if call := calls[1]; call.srcFileType != mountSourcePlain ||
		call.target != dir2 || call.dstFd == "" || call.fstype != "" || call.flags != unix.MS_BIND || call.data != "" {
		t.Fatalf("unexpected shared tmpfs bind mount call: %#v", call)
	}
	if call := calls[2]; call.srcFileName != "/dev/null" || call.srcFileType != mountSourcePlain ||
		call.target != file || call.dstFd == "" || call.fstype != "" || call.flags != unix.MS_BIND || call.data != "" {
		t.Fatalf("unexpected file mask mount call: %#v", call)
	}
}

func TestIsProcFdPath(t *testing.T) {
	for _, path := range []string{
		"/proc/thread-self/fd/7",
		"/proc/self/fd/7",
		"/proc/self/task/123/fd/7",
		"/proc/self/task/123/fd/../fd/7",
		"/proc/123/fd/7",
		"/proc/123/task/456/fd/7",
	} {
		if !isProcFdPath(path) {
			t.Errorf("expected %q to be a procfd path", path)
		}
	}
	for _, path := range []string{
		"/proc/acpi",
		"/proc/self/fdinfo/7",
		"/proc/self/task/123/fdinfo/7",
		"/proc/self/task/foo/fd/7",
		"/proc/foo/fd/7",
		"/proc/123/task/foo/fd/7",
		"/sys/devices/system/cpu/cpu0/thermal_throttle",
	} {
		if isProcFdPath(path) {
			t.Errorf("expected %q not to be a procfd path", path)
		}
	}
}

func TestMaskPathsDoesNotReuseProcFdMaskAsSharedSource(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(root, "dir2")
	dir3 := filepath.Join(root, "dir3")
	rootFd, err := os.OpenFile(root, unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_PATH, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFd.Close()
	for _, dir := range []string{dir1, dir2, dir3} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dir1File, err := os.OpenFile(dir1, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer dir1File.Close()
	procFd, closer := utils.ProcThreadSelfFd(dir1File.Fd())
	defer closer()

	old := mountFn
	t.Cleanup(func() { mountFn = old })
	var calls []recordedMount
	mountFn = recordMounts(&calls)
	if err := maskPaths(rootFd, []string{procFd, dir2, dir3}, ""); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 {
		t.Fatalf("expected 3 mount calls, got %d: %#v", len(calls), calls)
	}
	for _, call := range calls[:2] {
		if call.fstype != "tmpfs" || call.flags != unix.MS_RDONLY {
			t.Fatalf("expected procfd source to force separate tmpfs mounts, got %#v", calls)
		}
	}
	if call := calls[2]; call.srcFileType != mountSourcePlain ||
		call.target != dir3 || call.fstype != "" || call.flags != unix.MS_BIND {
		t.Fatalf("expected third directory to bind mount from second directory, got %#v", call)
	}
}

func TestMaskPathsDirBindFallback(t *testing.T) {
	root := t.TempDir()
	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(root, "dir2")
	dir3 := filepath.Join(root, "dir3")
	rootFd, err := os.OpenFile(root, unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_PATH, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rootFd.Close()
	for _, dir := range []string{dir1, dir2, dir3} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	old := mountFn
	t.Cleanup(func() { mountFn = old })
	var calls []recordedMount
	base := recordMounts(&calls)
	mountFn = func(source string, srcFile *mountSource, target, dstFd, fstype string, flags uintptr, data string) error {
		if err := base(source, srcFile, target, dstFd, fstype, flags, data); err != nil {
			return err
		}
		// Fail bind-mounts of the shared dir source to trigger fallback.
		if flags&unix.MS_BIND != 0 && srcFile != nil {
			return errors.New("bind mount not supported")
		}
		return nil
	}

	if err := maskPaths(rootFd, []string{dir1, dir2, dir3}, ""); err != nil {
		t.Fatal(err)
	}

	// Expected sequence:
	//   call[0]: tmpfs on dir1 (establishes shared source)
	//   call[1]: bind on dir2 -> FAILS (triggers bindFailed=true)
	//   call[2]: tmpfs on dir2 (fallback)
	//   call[3]: tmpfs on dir3 (bindFailed=true, no bind attempt)
	if len(calls) != 4 {
		t.Fatalf("expected 4 mount calls, got %d: %#v", len(calls), calls)
	}
	if call := calls[0]; call.fstype != "tmpfs" || call.flags != unix.MS_RDONLY || call.target != dir1 {
		t.Errorf("call[0]: expected tmpfs on dir1, got %#v", call)
	}
	if call := calls[1]; call.flags != unix.MS_BIND || call.srcFileType != mountSourcePlain || call.target != dir2 {
		t.Errorf("call[1]: expected failed bind attempt on dir2, got %#v", call)
	}
	if call := calls[2]; call.fstype != "tmpfs" || call.flags != unix.MS_RDONLY || call.target != dir2 {
		t.Errorf("call[2]: expected tmpfs fallback on dir2, got %#v", call)
	}
	if call := calls[3]; call.fstype != "tmpfs" || call.flags != unix.MS_RDONLY || call.target != dir3 {
		t.Errorf("call[3]: expected tmpfs on dir3, got %#v", call)
	}
}

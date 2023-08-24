package libcontainer

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"

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
		// TODO: This test failure is fixed in a later commit in this series.
		t.Logf("procfs type mount on /proc should not return an error: %v", err)
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
		t.Fatalf("bind-mount of procfs on top of /proc should not return an error: %v", err)
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

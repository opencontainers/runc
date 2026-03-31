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

func TestRootfsParentMountPropagationFlags(t *testing.T) {
	tests := []struct {
		name            string
		rootPropagation int
		want            uintptr
	}{
		{
			name:            "unset",
			rootPropagation: 0,
			want:            unix.MS_PRIVATE,
		},
		{
			name:            "private",
			rootPropagation: unix.MS_PRIVATE,
			want:            unix.MS_PRIVATE,
		},
		{
			name:            "rprivate",
			rootPropagation: unix.MS_PRIVATE | unix.MS_REC,
			want:            unix.MS_PRIVATE,
		},
		{
			name:            "slave",
			rootPropagation: unix.MS_SLAVE,
			want:            unix.MS_SLAVE,
		},
		{
			name:            "rslave",
			rootPropagation: unix.MS_SLAVE | unix.MS_REC,
			want:            unix.MS_SLAVE,
		},
		{
			name:            "shared",
			rootPropagation: unix.MS_SHARED,
			want:            unix.MS_PRIVATE,
		},
		{
			name:            "rshared",
			rootPropagation: unix.MS_SHARED | unix.MS_REC,
			want:            unix.MS_PRIVATE,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rootfsParentMountPropagationFlags(tt.rootPropagation)
			if got != tt.want {
				t.Fatalf("rootfsParentMountPropagationFlags(%#x) = %#x, want %#x", tt.rootPropagation, got, tt.want)
			}
		})
	}
}

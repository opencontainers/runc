// +build linux

package libcontainer

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestCheckMountDestOnProc(t *testing.T) {
	dest := "/rootfs/proc/"
	err := checkMountDestination("/rootfs", dest)
	if err == nil {
		t.Fatal("destination inside proc should return an error")
	}
}

func TestCheckMountDestInSys(t *testing.T) {
	dest := "/rootfs//sys/fs/cgroup"
	err := checkMountDestination("/rootfs", dest)
	if err != nil {
		t.Fatal("destination inside /sys should not return an error")
	}
}

func TestCheckMountDestFalsePositive(t *testing.T) {
	dest := "/rootfs/sysfiles/fs/cgroup"
	err := checkMountDestination("/rootfs", dest)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCheckMountRoot(t *testing.T) {
	dest := "/rootfs"
	err := checkMountDestination("/rootfs", dest)
	if err == nil {
		t.Fatal(err)
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
	setupDev, err := needsSetupDev(config)
	if err != nil {
		t.Fatal(err)
	}
	if setupDev {
		t.Fatalf("expected false")
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
	setupDev, err := needsSetupDev(config)
	if err != nil {
		t.Fatal(err)
	}
	if setupDev {
		t.Fatalf("expected false")
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
	setupDev, err := needsSetupDev(config)
	if err != nil {
		t.Fatal(err)
	}
	if !setupDev {
		t.Fatalf("expected true")
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
	setupDev, err := needsSetupDev(config)
	if err != nil {
		t.Fatal(err)
	}
	if !setupDev {
		t.Fatalf("expected true")
	}
}

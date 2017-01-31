// +build linux,selinux

package selinux_test

import (
	"os"
	"testing"

	"github.com/opencontainers/runc/libcontainer/selinux"
)

func TestSetfilecon(t *testing.T) {
	if selinux.SelinuxEnabled() {
		tmp := "selinux_test"
		con := "system_u:object_r:bin_t:s0"
		out, _ := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE, 0)
		out.Close()
		err := selinux.Setfilecon(tmp, con)
		if err != nil {
			t.Log("Setfilecon failed")
			t.Fatal(err)
		}
		filecon, err := selinux.Getfilecon(tmp)
		if err != nil {
			t.Log("Getfilecon failed")
			t.Fatal(err)
		}
		if con != filecon {
			t.Fatal("Getfilecon failed, returned %s expected %s", filecon, con)
		}

		os.Remove(tmp)
	}
}

func TestSELinux(t *testing.T) {
	var (
		err            error
		plabel, flabel string
	)

	if selinux.SelinuxEnabled() {
		t.Log("Enabled")
		plabel, flabel = selinux.GetLxcContexts()
		t.Log(plabel)
		t.Log(flabel)
		selinux.FreeLxcContexts(plabel)
		plabel, flabel = selinux.GetLxcContexts()
		t.Log(plabel)
		t.Log(flabel)
		selinux.FreeLxcContexts(plabel)
		t.Log("getenforce ", selinux.SelinuxGetEnforce())
		mode := selinux.SelinuxGetEnforceMode()
		t.Log("getenforcemode ", mode)

		defer selinux.SelinuxSetEnforce(mode)
		if err := selinux.SelinuxSetEnforce(selinux.Enforcing); err != nil {
			t.Fatalf("enforcing selinux failed: %v", err)
		}
		if err := selinux.SelinuxSetEnforce(selinux.Permissive); err != nil {
			t.Fatalf("setting selinux mode to permissive failed: %v", err)
		}
		selinux.SelinuxSetEnforce(mode)

		pid := os.Getpid()
		t.Logf("PID:%d MCS:%s\n", pid, selinux.IntToMcs(pid, 1023))
		err = selinux.Setfscreatecon("unconfined_u:unconfined_r:unconfined_t:s0")
		if err == nil {
			t.Log(selinux.Getfscreatecon())
		} else {
			t.Log("setfscreatecon failed", err)
			t.Fatal(err)
		}
		err = selinux.Setfscreatecon("")
		if err == nil {
			t.Log(selinux.Getfscreatecon())
		} else {
			t.Log("setfscreatecon failed", err)
			t.Fatal(err)
		}
		t.Log(selinux.Getpidcon(1))
		// Verify SELinux Containers Disabled works
		selinux.SetDisabled()
		if selinux.SelinuxEnabled() {
			t.Fatalf("SelinuxEnabled still is enabled after SELinux was disabled")
		}
		if !selinux.SelinuxEnabledHost() {
			t.Fatalf("SelinuxEnabledHost is no longer enabled after SELinux was disabled")
		}
		plabel, flabel = selinux.GetLxcContexts()
		if plabel != "" {
			t.Fatalf("GetLxcContext returned a process label on enabled system with container labeling disabled")
		}
		if flabel == "" {
			t.Fatalf("GetLxcContext did not return a file label on enabled system with container labeling disabled")
		}
		if selinux.SelinuxEnabled() {
			t.Fatalf("SelinuxEnabled still is enabled after SELinux was disabled")
		}

	} else {
		t.Log("Disabled")
	}
}

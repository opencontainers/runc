//go:build !linux
// +build !linux

package selinux

import (
	"testing"
)

const testLabel = "foobar"

func TestSELinuxStubs(t *testing.T) {
	if GetEnabled() {
		t.Error("SELinux enabled on non-linux.")
	}

	tmpDir := t.TempDir()
	if _, err := FileLabel(tmpDir); err != nil {
		t.Error(err)
	}

	if err := SetFileLabel(tmpDir, testLabel); err != nil {
		t.Error(err)
	}

	if _, err := LfileLabel(tmpDir); err != nil {
		t.Error(err)
	}
	if err := LsetFileLabel(tmpDir, testLabel); err != nil {
		t.Error(err)
	}

	if err := SetFSCreateLabel(testLabel); err != nil {
		t.Error(err)
	}

	if _, err := FSCreateLabel(); err != nil {
		t.Error(err)
	}
	if _, err := CurrentLabel(); err != nil {
		t.Error(err)
	}

	if _, err := PidLabel(0); err != nil {
		t.Error(err)
	}

	ClearLabels()

	ReserveLabel(testLabel)
	ReleaseLabel(testLabel)
	if _, err := DupSecOpt(testLabel); err != nil {
		t.Error(err)
	}
	if v := DisableSecOpt(); len(v) != 1 || v[0] != "disable" {
		t.Errorf(`expected "disabled", got %v`, v)
	}
	SetDisabled()
	if enabled := GetEnabled(); enabled {
		t.Error("Should not be enabled")
	}
	if err := SetExecLabel(testLabel); err != nil {
		t.Error(err)
	}
	if err := SetTaskLabel(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := ExecLabel(); err != nil {
		t.Error(err)
	}
	if _, err := CanonicalizeContext(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := ComputeCreateContext("foo", "bar", testLabel); err != nil {
		t.Error(err)
	}
	if err := SetSocketLabel(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := ClassIndex(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := SocketLabel(); err != nil {
		t.Error(err)
	}
	if _, err := PeerLabel(0); err != nil {
		t.Error(err)
	}
	if err := SetKeyLabel(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := KeyLabel(); err != nil {
		t.Error(err)
	}
	if err := SetExecLabel(testLabel); err != nil {
		t.Error(err)
	}
	if _, err := ExecLabel(); err != nil {
		t.Error(err)
	}
	con, err := NewContext(testLabel)
	if err != nil {
		t.Error(err)
	}
	con.Get()
	if err = SetEnforceMode(1); err != nil {
		t.Error(err)
	}
	if v := DefaultEnforceMode(); v != Disabled {
		t.Errorf("expected %d, got %d", Disabled, v)
	}
	if v := EnforceMode(); v != Disabled {
		t.Errorf("expected %d, got %d", Disabled, v)
	}
	if v := ROFileLabel(); v != "" {
		t.Errorf(`expected "", got %q`, v)
	}
	if processLbl, fileLbl := ContainerLabels(); processLbl != "" || fileLbl != "" {
		t.Errorf(`expected fileLbl="", fileLbl="" got processLbl=%q, fileLbl=%q`, processLbl, fileLbl)
	}
	if err = SecurityCheckContext(testLabel); err != nil {
		t.Error(err)
	}
	if _, err = CopyLevel("foo", "bar"); err != nil {
		t.Error(err)
	}
}

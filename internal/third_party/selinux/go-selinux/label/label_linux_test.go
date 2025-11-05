package label

import (
	"errors"
	"os"
	"testing"

	"github.com/opencontainers/selinux/go-selinux"
)

func needSELinux(t *testing.T) {
	t.Helper()
	if !selinux.GetEnabled() {
		t.Skip("SELinux not enabled, skipping.")
	}
}

func TestInit(t *testing.T) {
	needSELinux(t)

	var testNull []string
	_, _, err := InitLabels(testNull)
	if err != nil {
		t.Fatalf("InitLabels failed: %v:", err)
	}
	testDisabled := []string{"disable"}
	if selinux.ROFileLabel() == "" {
		t.Fatal("selinux.ROFileLabel: empty")
	}
	plabel, mlabel, err := InitLabels(testDisabled)
	if err != nil {
		t.Fatalf("InitLabels(disabled) failed: %v", err)
	}
	if plabel != "" {
		t.Fatalf("InitLabels(disabled): %q not empty", plabel)
	}
	if mlabel != "system_u:object_r:container_file_t:s0:c1022,c1023" {
		t.Fatalf("InitLabels Disabled mlabel Failed, %s", mlabel)
	}

	testUser := []string{"user:user_u", "role:user_r", "type:user_t", "level:s0:c1,c15"}
	plabel, mlabel, err = InitLabels(testUser)
	if err != nil {
		t.Fatalf("InitLabels(user) failed: %v", err)
	}
	if plabel != "user_u:user_r:user_t:s0:c1,c15" || (mlabel != "user_u:object_r:container_file_t:s0:c1,c15" && mlabel != "user_u:object_r:svirt_sandbox_file_t:s0:c1,c15") {
		t.Fatalf("InitLabels(user) failed (plabel=%q, mlabel=%q)", plabel, mlabel)
	}

	testBadData := []string{"user", "role:user_r", "type:user_t", "level:s0:c1,c15"}
	if _, _, err = InitLabels(testBadData); err == nil {
		t.Fatal("InitLabels(bad): expected error, got nil")
	}
}

func TestRelabel(t *testing.T) {
	needSELinux(t)

	testdir := t.TempDir()
	label := "system_u:object_r:container_file_t:s0:c1,c2"
	if err := Relabel(testdir, "", true); err != nil {
		t.Fatalf("Relabel with no label failed: %v", err)
	}
	if err := Relabel(testdir, label, true); err != nil {
		t.Fatalf("Relabel shared failed: %v", err)
	}
	if err := Relabel(testdir, label, false); err != nil {
		t.Fatalf("Relabel unshared failed: %v", err)
	}
	if err := Relabel("/etc", label, false); err == nil {
		t.Fatalf("Relabel /etc succeeded")
	}
	if err := Relabel("/", label, false); err == nil {
		t.Fatalf("Relabel / succeeded")
	}
	if err := Relabel("/usr", label, false); err == nil {
		t.Fatalf("Relabel /usr succeeded")
	}
	if err := Relabel("/usr/", label, false); err == nil {
		t.Fatalf("Relabel /usr/ succeeded")
	}
	if err := Relabel("/etc/passwd", label, false); err == nil {
		t.Fatalf("Relabel /etc/passwd succeeded")
	}
	if home := os.Getenv("HOME"); home != "" {
		if err := Relabel(home, label, false); err == nil {
			t.Fatalf("Relabel %s succeeded", home)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := Validate("zZ"); !errors.Is(err, ErrIncompatibleLabel) {
		t.Fatalf("Expected incompatible error, got %v", err)
	}
	if err := Validate("Z"); err != nil {
		t.Fatal(err)
	}
	if err := Validate("z"); err != nil {
		t.Fatal(err)
	}
	if err := Validate(""); err != nil {
		t.Fatal(err)
	}
}

func TestIsShared(t *testing.T) {
	if shared := IsShared("Z"); shared {
		t.Fatalf("Expected label `Z` to not be shared, got %v", shared)
	}
	if shared := IsShared("z"); !shared {
		t.Fatalf("Expected label `z` to be shared, got %v", shared)
	}
	if shared := IsShared("Zz"); !shared {
		t.Fatalf("Expected label `Zz` to be shared, got %v", shared)
	}
}

func TestFileLabel(t *testing.T) {
	needSELinux(t)

	testUser := []string{"filetype:test_file_t", "level:s0:c1,c15"}
	_, mlabel, err := InitLabels(testUser)
	if err != nil {
		t.Fatalf("InitLabels(user) failed: %v", err)
	}
	if mlabel != "system_u:object_r:test_file_t:s0:c1,c15" {
		t.Fatalf("InitLabels(filetype) failed: %v", err)
	}
}

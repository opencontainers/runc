//go:build !linux
// +build !linux

package label

import (
	"testing"

	"github.com/opencontainers/selinux/go-selinux"
)

const testLabel = "system_u:object_r:container_file_t:s0:c1,c2"

func TestInit(t *testing.T) {
	var testNull []string
	_, _, err := InitLabels(testNull)
	if err != nil {
		t.Log("InitLabels Failed")
		t.Fatal(err)
	}
	testDisabled := []string{"disable"}
	if selinux.ROFileLabel() != "" {
		t.Error("selinux.ROFileLabel Failed")
	}
	plabel, mlabel, err := InitLabels(testDisabled)
	if err != nil {
		t.Log("InitLabels Disabled Failed")
		t.Fatal(err)
	}
	if plabel != "" {
		t.Fatal("InitLabels Disabled Failed")
	}
	if mlabel != "" {
		t.Fatal("InitLabels Disabled mlabel Failed")
	}
	testUser := []string{"user:user_u", "role:user_r", "type:user_t", "level:s0:c1,c15"}
	_, _, err = InitLabels(testUser)
	if err != nil {
		t.Log("InitLabels User Failed")
		t.Fatal(err)
	}
}

func TestRelabel(t *testing.T) {
	if err := Relabel("/etc", testLabel, false); err != nil {
		t.Fatalf("Relabel /etc succeeded")
	}
}

func TestCheckLabelCompile(t *testing.T) {
	if _, _, err := InitLabels(nil); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()

	if err := SetFileLabel(tmpDir, "foobar"); err != nil {
		t.Fatal(err)
	}

	if err := SetFileCreateLabel("foobar"); err != nil {
		t.Fatal(err)
	}

	DisableSecOpt()

	if err := Validate("foobar"); err != nil {
		t.Fatal(err)
	}
	if relabel := RelabelNeeded("foobar"); relabel {
		t.Fatal("Relabel failed")
	}
	if shared := IsShared("foobar"); shared {
		t.Fatal("isshared failed")
	}
}

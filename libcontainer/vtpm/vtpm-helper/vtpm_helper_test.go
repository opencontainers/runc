// + build linux

package vtpmhelper

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/vtpm"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestCreateVTPMFail(t *testing.T) {
	vtpmdev := specs.LinuxVTPM{}

	_, err := CreateVTPM(&specs.Spec{}, &vtpmdev, 0)
	if err == nil {
		t.Fatalf("Could create vTPM without statepath %v", err)
	}
}

// check prerequisites for starting a vTPM
func checkPrerequisites(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("Need to be root to run this test")
	}

	for _, executable := range []string{"swtpm_setup", "swtpm"} {
		if err := exec.Command(executable, "--help").Run(); err != nil {
			t.Skipf("Could not run %s --help: %v", executable, err)
		}
	}
}

func createVTPM(t *testing.T, tpmversion string, createCertificates bool, runas string) *vtpm.VTPM {

	checkPrerequisites(t)

	workdir, err := ioutil.TempDir("", "runctest")
	if err != nil {
		t.Fatalf("Could not create tmp dir: %s", err)
	}
	defer os.Remove(workdir)

	tpmdirname := path.Join(workdir, "myvtpm")

	spec := &specs.Spec{
		Linux: &specs.Linux{
			Devices:   []specs.LinuxDevice{},
			Resources: &specs.LinuxResources{},
		},
	}
	vtpmdev := &specs.LinuxVTPM{
		StatePath:          tpmdirname,
		TPMVersion:         tpmversion,
		CreateCertificates: createCertificates,
		RunAs:              runas,
	}

	myvtpm, err := CreateVTPM(spec, vtpmdev, 0)
	if err != nil {
		if strings.Contains(err.Error(), "VTPM device driver not available") {
			t.Skipf("%v", err)
		} else {
			t.Fatalf("Could not create VTPM device: %v", err)
		}
	}
	return myvtpm
}

func destroyVTPM(t *testing.T, myvtpm *vtpm.VTPM) {
	tpmdirname := myvtpm.StatePath

	DestroyVTPMs([]*vtpm.VTPM{myvtpm})

	if _, err := os.Stat(tpmdirname); !os.IsNotExist(err) {
		t.Fatalf("State directory should have been removed since it was created by vtpm-helpers")
	}
}

func createRestartDestroyVTPM(t *testing.T, tpmversion string, createCertificates bool, runas string) {
	myvtpm := createVTPM(t, tpmversion, createCertificates, runas)

	err := myvtpm.Stop(false)
	if err != nil {
		t.Fatalf("VTPM could not be stopped cleanly: %v", err)
	}

	createdStatePath, err := myvtpm.Start()
	if err != nil {
		t.Fatalf("VTPM could not be started: %v", err)
	}
	if createdStatePath {
		t.Fatalf("VTPM Start() should not have created the state path at this time")
	}

	destroyVTPM(t, myvtpm)
}

func TestCreateVTPM2(t *testing.T) {
	createRestartDestroyVTPM(t, "", true, "root")
	createRestartDestroyVTPM(t, "", false, "0")
	createRestartDestroyVTPM(t, "2", true, "0")
}

func TestCreateVTPM12(t *testing.T) {
	createRestartDestroyVTPM(t, "1.2", true, "root")
}

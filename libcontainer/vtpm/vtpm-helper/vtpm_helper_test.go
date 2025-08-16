// + build linux

package vtpmhelper

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/vtpm"
	"github.com/opencontainers/runtime-spec/specs-go"
)

func TestCreateVTPMFail(t *testing.T) {
	vtpmdev := specs.LinuxVTPM{}

	_, err := CreateVTPM(&specs.Spec{}, &vtpmdev)
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

const (
	majorEnvName = "RUN_IN_CONTAINER_MAJOR"
	minorEnvName = "RUN_IN_CONTAINER_MINOR"
)

func getDefaultMajorMinorDevices() (uint32, uint32, error) {
	var major, minor uint32
	if val := os.Getenv(majorEnvName); len(val) > 0 {
		converted, err := strconv.Atoi(val)
		if err != nil {
			return 0, 0, fmt.Errorf("can not use %s as a device major: %s", val, err)
		}
		major = uint32(converted)
	}
	if val := os.Getenv(minorEnvName); len(val) > 0 {
		converted, err := strconv.Atoi(val)
		if err != nil {
			return 0, 0, fmt.Errorf("can not use %s as a device minor: %s", val, err)
		}
		minor = uint32(converted)
	}
	return major, minor, nil
}

func createVTPM(t *testing.T, tpmversion string, createCertificates bool, runas, encryptionPassword string) *vtpm.VTPM {

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

	major, minor, err := getDefaultMajorMinorDevices()
	if err != nil {
		t.Fatalf("Could not get default device major, minor: %s", err)
	}

	vtpmdev := &specs.LinuxVTPM{
		StatePath:          tpmdirname,
		VTPMVersion:        tpmversion,
		CreateCertificates: createCertificates,
		RunAs:              runas,
		EncryptionPassword: encryptionPassword,
		VTPMMajor:          major,
		VTPMMinor:          minor,
	}

	myvtpm, err := CreateVTPM(spec, vtpmdev)
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
	if err := os.Remove(myvtpm.GetTPMDevpath()); err != nil && !os.IsNotExist(err) {
		t.Fatalf("While testing the docker container, we should remove device ourselves: %s", err)
	}
}

func createRestartDestroyVTPM(t *testing.T, tpmversion string, createCertificates bool, runas, encryptionPassword string) {
	myvtpm := createVTPM(t, tpmversion, createCertificates, runas, encryptionPassword)

	err := myvtpm.Stop(false)
	if err != nil {
		t.Fatalf("VTPM could not be stopped cleanly: %v", err)
	}

	if err := os.Remove(myvtpm.GetTPMDevpath()); err != nil && !os.IsNotExist(err) {
		t.Fatalf("While testing the docker container, we should remove device ourselves: %s", err)
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
	createRestartDestroyVTPM(t, "", true, "root", "")
	createRestartDestroyVTPM(t, "", false, "0", "")
	createRestartDestroyVTPM(t, "2", true, "0", "")
}

func TestCreateVTPM12(t *testing.T) {
	createRestartDestroyVTPM(t, "1.2", true, "root", "")
}

func TestCreateEncryptedVTPM_Pipe(t *testing.T) {
	checkPrerequisites(t)

	piper, pipew, err := os.Pipe()
	if err != nil {
		t.Fatalf("Could not create pipe")
	}
	defer piper.Close()

	password := "123456"

	// pass password via write to pipe
	go func() {
		n, err := pipew.Write([]byte(password))
		if err != nil {
			t.Fatalf("Could not write to pipe: %v", err)
		}
		if n != len(password) {
			t.Fatalf("Could not write all data to pipe")
		}
		pipew.Close()
	}()
	createRestartDestroyVTPM(t, "", true, "root", fmt.Sprintf("fd=%d", piper.Fd()))
}

func TestCreateEncryptedVTPM_File(t *testing.T) {
	fil, err := ioutil.TempFile("", "passwordfile")
	if err != nil {
		t.Fatalf("Could not create temporary file: %v", err)
	}
	defer os.Remove(fil.Name())

	_, err = fil.WriteString("123456")
	if err != nil {
		t.Fatalf("Could not write to temporary file: %v", err)
	}
	createRestartDestroyVTPM(t, "", true, "root", fmt.Sprintf("file=%s", fil.Name()))
}

func TestCreateEncryptedVTPM_Direct(t *testing.T) {
	createRestartDestroyVTPM(t, "", true, "root", "pass=123456")
	createRestartDestroyVTPM(t, "", true, "root", "123456")
}

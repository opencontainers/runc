// + build linux

package vtpmhelper

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/vtpm"

	"github.com/opencontainers/runtime-spec/specs-go"
	// "golang.org/x/sys/unix"
	// "github.com/sirupsen/logrus"
)


// getEncryptionPassword gets the plain password from the caller
// valid formats passed to this function are:
// - <password>
// - pass=<password>
// - fd=<filedescriptor>
// - file=<filename>
func getEncryptionPassword(pwdString string) ([]byte, error) {
	if strings.HasPrefix(pwdString, "file=") {
		return ioutil.ReadFile(pwdString[5:])
	} else if strings.HasPrefix(pwdString, "pass=") {
		return []byte(pwdString[5:]), nil
	} else if strings.HasPrefix(pwdString, "fd=") {
		fdStr := pwdString[3:]
		fd, err := strconv.Atoi(fdStr)
		if err != nil {
			return nil, fmt.Errorf("could not parse file descriptor %s", fdStr)
		}
		f := os.NewFile(uintptr(fd), "pwdfile")
		if f == nil {
			return nil, fmt.Errorf("%s is not a valid file descriptor", fdStr)
		}
		defer f.Close()
		pwd := make([]byte, 1024)
		n, err := f.Read(pwd)
		if err != nil {
			return nil, fmt.Errorf("could not read from file descriptor: %v", err)
		}
		return pwd[:n], nil
	}
	return []byte(pwdString), nil
}

// CreateVTPM create a VTPM proxy device and starts the TPM emulator with it
func CreateVTPM(spec *specs.Spec, vtpmdev *specs.LinuxVTPM) (*vtpm.VTPM, error) {

	encryptionPassword, err := getEncryptionPassword(vtpmdev.EncryptionPassword)
	if err != nil {
		return nil, err
	}

	vtpm, err := vtpm.NewVTPM(vtpmdev, encryptionPassword)
	if err != nil {
		return nil, err
	}

	// Start the vTPM process; once stopped, the device pair will also disappear
	vtpm.CreatedStatepath, err = vtpm.Start()
	if err != nil {
		return nil, err
	}

	return vtpm, nil
}

func setVTPMHostDevOwner(vtpm *vtpm.VTPM, uid, gid int) error {
	hostdev := vtpm.GetTPMDevpath()
	// adapt ownership of the device since only root can access it
	if err := os.Chown(hostdev, uid, gid); err != nil {
		return err
	}

	host_tpmrm := fmt.Sprintf("/dev/tpmrm%s", vtpm.Tpm_dev_num)
	if _, err := os.Lstat(host_tpmrm); err == nil {
		// adapt ownership of the device since only root can access it
		if err := os.Chown(host_tpmrm, uid, gid); err != nil {
			return err
		}
	}

	return nil
}

// SetVTPMHostDevsOwner sets the owner of the host devices to the
// container root's mapped user id; if root inside the container is
// uid 1000 on the host, the devices will be owned by uid 1000.
func SetVTPMHostDevsOwner(config *configs.Config, uid, gid int) error {
	if uid != 0 {
		for _, vtpm := range config.VTPMs {
			if err := setVTPMHostDevOwner(vtpm, uid, gid); err != nil {
				return err
			}
		}
	}
	return nil
}

// DestroyVTPMs stops all VTPMs and cleans up the state directory if necessary
func DestroyVTPMs(vtpms []*vtpm.VTPM) {
	for _, vtpm := range vtpms {
		vtpm.Stop(vtpm.CreatedStatepath)
	}
}

// ApplyCGroupVTPMs puts all VTPMs into the given Cgroup manager's cgroup
func ApplyCGroupVTPMs(vtpms []*vtpm.VTPM, cgroupManager cgroups.Manager) error {
	for _, vtpm := range vtpms {
		if err := cgroupManager.Apply(vtpm.Pid); err != nil {
			return fmt.Errorf("cGroupManager failed to apply vtpm with pid %d: %v", vtpm.Pid, err)
		}
	}
	return nil
}

// + build linux

package vtpmhelper

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/vtpm"

	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// addVTPMDevice adds a device and cgroup entry to the spec
func addVTPMDevice(spec *specs.Spec, hostpath, devpath string, major, minor uint32) {
	var filemode os.FileMode = 0600

	device := specs.LinuxDevice{
		Path:     hostpath,
		Devpath:  devpath,
		Type:     "c",
		Major:    int64(major),
		Minor:    int64(minor),
		FileMode: &filemode,
	}
	spec.Linux.Devices = append(spec.Linux.Devices, device)

	major_p := new(int64)
	*major_p = int64(major)
	minor_p := new(int64)
	*minor_p = int64(minor)

	ld := &specs.LinuxDeviceCgroup{
		Allow:  true,
		Type:   "c",
		Major:  major_p,
		Minor:  minor_p,
		Access: "rwm",
	}
	spec.Linux.Resources.Devices = append(spec.Linux.Resources.Devices, *ld)
}

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
func CreateVTPM(spec *specs.Spec, vtpmdev *specs.LinuxVTPM, devnum int) (*vtpm.VTPM, error) {
	encryptionPassword, err := getEncryptionPassword(vtpmdev.EncryptionPassword)
	if err != nil {
		return nil, err
	}

	vtpm, err := vtpm.NewVTPM(vtpmdev.StatePath, vtpmdev.StatePathIsManaged, vtpmdev.TPMVersion, vtpmdev.CreateCertificates, vtpmdev.RunAs, vtpmdev.PcrBanks, encryptionPassword)
	if err != nil {
		return nil, err
	}

	// Start the vTPM process; once stopped, the device pair will also disappear
	vtpm.CreatedStatepath, err = vtpm.Start()
	if err != nil {
		return nil, err
	}

	hostdev := vtpm.GetTPMDevname()
	major, minor := vtpm.GetMajorMinor()

	devpath := fmt.Sprintf("/dev/tpm%d", devnum)
	addVTPMDevice(spec, hostdev, devpath, major, minor)

	// for TPM 2: check if /dev/vtpmrm%d is available
	host_tpmrm := fmt.Sprintf("/dev/tpmrm%d", vtpm.GetTPMDevNum())
	if fileInfo, err := os.Lstat(host_tpmrm); err == nil {
		if stat_t, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			devNumber := stat_t.Rdev
			devpath = fmt.Sprintf("/dev/tpmrm%d", devnum)
			addVTPMDevice(spec, host_tpmrm, devpath, unix.Major(devNumber), unix.Minor(devNumber))
		}
	}

	return vtpm, nil
}

func setVTPMHostDevOwner(vtpm *vtpm.VTPM, uid, gid int) error {
	hostdev := vtpm.GetTPMDevname()
	// adapt ownership of the device since only root can access it
	if err := os.Chown(hostdev, uid, gid); err != nil {
		return err
	}

	host_tpmrm := fmt.Sprintf("/dev/tpmrm%d", vtpm.GetTPMDevNum())
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

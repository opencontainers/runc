// + build linux

package vtpmhelper

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/opencontainers/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/vtpm"

	"github.com/opencontainers/runtime-spec/specs-go"
	// "golang.org/x/sys/unix"
	"github.com/sirupsen/logrus"
)

const (
	HashedRootOffset = 6
)

// getEncryptionPassword gets the plain password from the caller
// valid formats passed to this function are:
// - <password>
// - pass=<password>
// - fd=<filedescriptor>
// - file=<filename>
func getEncryptionPassword(pwdString string) ([]byte, error) {
	switch {
	case strings.HasPrefix(pwdString, "file="):
		return ioutil.ReadFile(pwdString[5:])
	case strings.HasPrefix(pwdString, "pass="):
		return []byte(pwdString[5:]), nil
	case strings.HasPrefix(pwdString, "fd="):
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
	default:
		return []byte(pwdString), nil
	}
}

func GenerateDeviceHostPathName(root, containerName, deviceName string) string {
	// In runc we do not have a namespace, so we will used hash from root as a namespace.
	hashedValue := fmt.Sprintf("%x", sha256.Sum256(append([]byte(root), '\n')))
	return fmt.Sprintf("%s-%s-%s", hashedValue[:HashedRootOffset], containerName, deviceName)
}

// CreateVTPM create a VTPM proxy device and starts the TPM emulator with it
func CreateVTPM(spec *specs.Spec, vtpmdev *specs.LinuxVTPM) (*vtpm.VTPM, error) {
	encryptionPassword, err := getEncryptionPassword(vtpmdev.EncryptionPassword)
	if err != nil {
		return nil, fmt.Errorf("can not get encryption password: %w", err)
	}

	vtpm, err := vtpm.NewVTPM(vtpmdev, encryptionPassword)
	if err != nil {
		return nil, fmt.Errorf("can not create new vtpm: %w", err)
	}

	// Start the vTPM process; once stopped, the device pair will also disappear
	vtpm.CreatedStatepath, err = vtpm.Start()
	if err != nil {
		return nil, fmt.Errorf("can not start new vtpm: %w", err)
	}

	return vtpm, nil
}

func setVTPMHostDevOwner(vtpm *vtpm.VTPM, uid, gid int) error {
	hostdev := vtpm.GetTPMDevpath()
	// adapt ownership of the device since only root can access it
	if err := os.Chown(hostdev, uid, gid); err != nil {
		return err
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
		pathes := cgroupManager.GetPaths()
		for subsys, path := range pathes {
			if err := cgroups.WriteCgroupProc(path, vtpm.Pid); err != nil {
				return fmt.Errorf("cGroupManager failed to apply vtpm %s subsys with pid %d: %v", subsys, vtpm.Pid, err)
			}
		}
	}
	return nil
}

func CheckVTPMNames(vtpms []string) error {
	namesMap := make(map[string]int, 0)
	for ind, name := range vtpms {
		if name == "" {
			return fmt.Errorf("VTPM device %d has empty name", ind)
		}
		if mappedInd, ok := namesMap[name]; ok {
			return fmt.Errorf("VTPM devices %d and %d has the same name %s", mappedInd, ind, name)
		}
		namesMap[name] = ind
	}
	return nil
}

const defaultSWTPMRuncConfig = "/etc/swtpm/runc.conf"

var (
	ignoreVtpmErrors bool
	configOnce       sync.Once
)

func CanIgnoreVTPMErrors() bool {
	configOnce.Do(func() {
		file, err := os.Open(defaultSWTPMRuncConfig)
		if err != nil {
			logrus.Errorf("can not open config %s: %s", defaultSWTPMRuncConfig, err)
			return
		}

		data, err := io.ReadAll(file)
		if err != nil {
			logrus.Errorf("can not read data from config %s: %s", defaultSWTPMRuncConfig, err)
			return
		}

		var config struct {
			IgnoreVTPMErrors bool `json:"ignoreVTPMErrors,omitempty"`
		}

		err = json.Unmarshal(data, &config)
		if err != nil {
			logrus.Errorf("can not unmarshal config %s: %s", defaultSWTPMRuncConfig, err)
			return
		}
		ignoreVtpmErrors = config.IgnoreVTPMErrors
	})
	return ignoreVtpmErrors
}

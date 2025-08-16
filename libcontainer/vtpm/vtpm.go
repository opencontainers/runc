// + build linux

package vtpm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/opencontainers/runc/libcontainer/apparmor"
	"github.com/opencontainers/runtime-spec/specs-go"
	selinux "github.com/opencontainers/selinux/go-selinux"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// object
type VTPM struct {
	// name of vtpm
	VtpmName string `json:"vtpmName"`

	// The path where the TPM emulator writes the TPM state to
	StatePath string `json:"statePath"`

	// Whether we are allowed to delete the TPM's state path upon
	// destroying the TPM or an outside mgmt. stack will do that
	StatePathIsManaged bool `json:"statePathIsManaged"`

	// whether the device state path was created or already existed
	CreatedStatepath bool

	// Whether to create a certificate for the VTPM
	CreateCerts bool `json:"createCerts"`

	// Version of the TPM
	Vtpmversion string `json:"vtpmversion"`

	// Set of active PCR banks
	PcrBanks string `json:"pcrbanks"`

	// plain text encryption password used by vTPM
	encryptionPassword []byte

	// whether an error occurred writing the password to the pipe
	passwordPipeError error

	// The user under which to run the TPM emulator
	user string

	// The major number of the created device
	major uint32

	// The minor number of the created device
	minor uint32

	// process id of this vtpm
	Pid int

	// The AppArmor profile's full path
	aaprofile string

	// swtpm_setup capabilities
	swtpmSetupCaps []string

	// swtpm capabilities
	swtpmCaps []string
}

const (
	VTPM_VERSION_1_2 = "1.2"
	VTPM_VERSION_2   = "2"
)

func translateUser(username string) (*user.User, error) {
	usr, err := user.Lookup(username)
	if err != nil {
		usr, err = user.LookupId(username)
	}
	if err != nil {
		return nil, fmt.Errorf("User '%s' not available: %v", username, err)
	}
	return usr, nil
}

// getCapabilities gets the capabilities map of an executable by invoking it with
// --print-capabilities. It returns the array of feature strings.
// This function returns an empty array if the executable does not support --print-capabilities.
// Expected output looks like this:
// { "type": "swtpm_setup", "features": [ "cmdarg-keyfile-fd", "cmdarg-pwdfile-fd" ] }
func getCapabilities(cmd *exec.Cmd) ([]string, error) {
	caps := make(map[string]interface{})

	output, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	err = json.Unmarshal([]byte(output), &caps)
	if err != nil {
		return nil, fmt.Errorf("Could not unmarshal output: %s: %v\n", output, err)
	}

	features, _ := caps["features"].([]interface{})
	res := make([]string, 0)
	for _, f := range features {
		res = append(res, f.(string))
	}
	return res, nil
}

func getSwtpmSetupCapabilities() ([]string, error) {
	return getCapabilities(exec.Command("swtpm_setup", "--print-capabilities"))
}

func getSwtpmCapabilities() ([]string, error) {
	return getCapabilities(exec.Command("swtpm", "chardev", "--print-capabilities"))
}

func hasCapability(capabilities []string, capability string) bool {
	for _, c := range capabilities {
		if capability == c {
			return true
		}
	}
	return false
}

// Create a new VTPM object
//
// @statepath: directory where the vTPM's state will be written into
// @statepathismanaged: whether we are allowed to delete the TPM's state
//
//	path upon destroying the vTPM
//
// @vtpmversion: The TPM version
// @createcerts: whether to create certificates for the vTPM (on first start)
// @runas: the account under which to run the swtpm; TPM 1.2 should be run
//
//	with account tss; TPM 2 has more flexibility
//
// After successful creation of the object the Start() method can be called
func NewVTPM(vtpmdev *specs.LinuxVTPM, encryptionpassword []byte) (*VTPM, error) {
	vtpmname := vtpmdev.VTPMName
	statepath := vtpmdev.StatePath
	vtpmversion := vtpmdev.VTPMVersion
	runas := vtpmdev.RunAs
	createcerts := vtpmdev.CreateCertificates
	statepathismanaged := vtpmdev.StatePathIsManaged
	pcrbanks := vtpmdev.PcrBanks

	if len(statepath) == 0 {
		return nil, fmt.Errorf("Missing required statpath for vTPM.")
	}

	if len(vtpmversion) == 0 {
		vtpmversion = VTPM_VERSION_2
	}
	if vtpmversion != VTPM_VERSION_1_2 && vtpmversion != VTPM_VERSION_2 {
		return nil, fmt.Errorf("Unsupported VTPM version '%s'.", vtpmversion)
	}

	if runas == "" {
		runas = "root"
	}
	// TPM 1.2 choices are only 'root' and 'tss' users due to tcsd
	if vtpmversion == VTPM_VERSION_1_2 && runas != "root" && runas != "tss" {
		runas = "root"
	}

	usr, err := translateUser(runas)
	if err != nil {
		return nil, err
	}
	runas = usr.Uid

	swtpmSetupCaps, err := getSwtpmSetupCapabilities()
	if err != nil {
		return nil, err
	}
	swtpmCaps, err := getSwtpmCapabilities()
	if err != nil {
		return nil, err
	}

	return &VTPM{
		user:               runas,
		StatePath:          statepath,
		StatePathIsManaged: statepathismanaged,
		Vtpmversion:        vtpmversion,
		CreateCerts:        createcerts,
		PcrBanks:           pcrbanks,
		encryptionPassword: encryptionpassword,
		swtpmSetupCaps:     swtpmSetupCaps,
		swtpmCaps:          swtpmCaps,
		VtpmName:           vtpmname,
		major:              uint32(vtpmdev.VTPMMajor),
		minor:              uint32(vtpmdev.VTPMMinor),
	}, nil
}

// getPidFile creates the full path of the TPM emulator PID file
func (vtpm *VTPM) getPidFile() string {
	return path.Join(vtpm.StatePath, vtpm.VtpmName+"-swtpm.pid")
}

// getLogFile creates the full path of the TPM emulator log file
func (vtpm *VTPM) getLogFile() string {
	return path.Join(vtpm.StatePath, "swtpm.log")
}

// getPidFromFile: Get the PID from the PID file
func (vtpm *VTPM) getPidFromFile() (int, error) {
	d, err := ioutil.ReadFile(vtpm.getPidFile())
	if err != nil {
		return -1, err
	}
	if len(d) == 0 {
		return -1, fmt.Errorf("Empty pid file")
	}

	pid, err := strconv.Atoi(string(d))
	if err != nil {
		return -1, fmt.Errorf("Could not parse pid from file: %s", string(d))
	}
	return pid, nil
}

// waitForPidFile: wait for the PID file to appear and read the PID from it
func (vtpm *VTPM) waitForPidFile(loops int) (int, error) {
	for loops >= 0 {
		pid, err := vtpm.getPidFromFile()
		if pid > 0 && err == nil {
			return pid, nil
		}
		time.Sleep(time.Millisecond * 100)
		loops -= 1
	}
	logrus.Error("PID file did not appear")
	return -1, fmt.Errorf("swtpm's pid file did not appear")
}

// stopByPidFile: Stop the vTPM by its PID file
func (vtpm *VTPM) stopByPidFile() error {

	pid, err := vtpm.getPidFromFile()
	if err != nil {
		return err
	}

	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	err = p.Signal(syscall.SIGTERM)

	return err
}

func (vtpm *VTPM) modifyModePath(dirPath string, mask, set os.FileMode) error {
	for {
		fileInfo, err := os.Stat(dirPath)
		if err != nil {
			return err
		}
		if !fileInfo.IsDir() {
			continue
		}

		mode := (fileInfo.Mode() & mask) | set
		if err := os.Chmod(dirPath, mode); err != nil {
			return err
		}

		dirPath = filepath.Dir(dirPath)
		if dirPath == "/" {
			break
		}
	}
	return nil
}

// DeleteStatePath deletes the directory where the TPM emulator writes its state
// into unless the state path is managed by a higher layer application, in which
// case the state path is not removed
func (vtpm *VTPM) DeleteStatePath() error {
	if !vtpm.StatePathIsManaged {
		return os.RemoveAll(vtpm.StatePath)
	}
	return nil
}

// createStatePath creates the TPM directory where the TPM writes its state
// into; it also makes the directory accessible to the 'runas' user
//
// This method returns true in case the path was created, false in case the
// path already existed
func (vtpm *VTPM) createStatePath() (bool, error) {
	created := false
	if _, err := os.Stat(vtpm.StatePath); err != nil {
		if err := os.MkdirAll(vtpm.StatePath, 0770); err != nil {
			return false, fmt.Errorf("Could not create directory %s: %v", vtpm.StatePath, err)
		}
		created = true
	}

	err := vtpm.chownStatePath()
	if err != nil {
		if created {
			vtpm.DeleteStatePath()
		}
		return false, err
	}
	return created, nil
}

func (vtpm *VTPM) chownStatePath() error {
	usr, err := translateUser(vtpm.user)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return fmt.Errorf("Error parsing Uid %s: %v", usr.Uid, err)
	}

	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return fmt.Errorf("Error parsing Gid %s: %v", usr.Gid, err)
	}

	err = filepath.Walk(vtpm.StatePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != vtpm.StatePath {
			return filepath.SkipDir
		}
		if err := os.Chown(path, uid, gid); err != nil {
			return fmt.Errorf("Could not change ownership of file %s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if uid != 0 {
		if err := vtpm.modifyModePath(vtpm.StatePath, 0777, 0010); err != nil {
			return fmt.Errorf("Could not chmod path to %s: %v", vtpm.StatePath, err)
		}
	}

	return nil
}

// setup the password pipe so that we can transfer the TPM state encryption via
// a pipe where the read-end is passed to swtpm / swtpm_setup as a file descriptor
func (vtpm *VTPM) setupPasswordPipe(password []byte) (*os.File, error) {
	if !hasCapability(vtpm.swtpmSetupCaps, "cmdarg-pwdfile-fd") {
		return nil, fmt.Errorf("Requiring newer version of swtpm for state encryption; needs cmdarg-pwd-fd feature")
	}

	piper, pipew, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("Could not create pipe")
	}
	vtpm.passwordPipeError = nil

	go func() {
		tot := 0
		for tot < len(password) {
			var n int
			n, vtpm.passwordPipeError = pipew.Write(password)
			if vtpm.passwordPipeError != nil {
				break
			}
			tot = tot + n
		}
		pipew.Close()
	}()
	return piper, nil
}

// runSwtpmSetup runs swtpm_setup to simulate TPM manufacturing by creating
// EK and platform certificates and enabling TPM 2 PCR banks
func (vtpm *VTPM) runSwtpmSetup() error {
	// if state already exists, --not-overwrite will not overwrite it
	cmd := exec.Command("swtpm_setup", "--tpm-state", vtpm.StatePath, "--createek",
		"--logfile", vtpm.getLogFile(), "--not-overwrite")
	if vtpm.Vtpmversion == VTPM_VERSION_1_2 {
		cmd.Args = append(cmd.Args, "--runas", vtpm.user)
	} else if vtpm.Vtpmversion == VTPM_VERSION_2 {
		// when creating certs we need root access to create lock files
		if !vtpm.CreateCerts {
			cmd.Args = append(cmd.Args, "--runas", vtpm.user)
		}
	}
	if vtpm.CreateCerts {
		cmd.Args = append(cmd.Args, "--create-ek-cert", "--create-platform-cert", "--lock-nvram")
	}
	if len(vtpm.encryptionPassword) > 0 {
		piper, err := vtpm.setupPasswordPipe(vtpm.encryptionPassword)
		if err != nil {
			return err
		}
		cmd.ExtraFiles = append(cmd.ExtraFiles, piper)
		pwdfile_fd := fmt.Sprintf("%d", 3+len(cmd.ExtraFiles)-1)
		cmd.Args = append(cmd.Args, "--cipher", "aes-256-cbc", "--pwdfile-fd", pwdfile_fd)
		defer piper.Close()
	}

	if vtpm.Vtpmversion == VTPM_VERSION_2 {
		cmd.Args = append(cmd.Args, "--tpm2")
		if len(vtpm.PcrBanks) > 0 {
			cmd.Args = append(cmd.Args, "--pcr-banks", vtpm.PcrBanks)
		}
	}

	// need to explicitly set TMPDIR
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TMPDIR=/tmp")

	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Errorf("swtpm_setup failed: %s", string(output))
		return fmt.Errorf("swtpm_setup failed: %s\nlog: %s", string(output), vtpm.ReadLog())
	}

	if vtpm.passwordPipeError != nil {
		return fmt.Errorf("Error transferring password using pipe: %v", vtpm.passwordPipeError)
	}

	return nil
}

// waitForTPMDevice: Wait for /dev/tpm%d to appear and while waiting
//
//	check whether the swtpm is still alive by checking its PID file
func (vtpm *VTPM) waitForTPMDevice(loops int) error {
	devpath := vtpm.GetTPMDevpath()
	pidfile := vtpm.getPidFile()

	for loops >= 0 {
		if _, err := os.Stat(pidfile); err != nil {
			logrus.Errorf("swtpm process has terminated")
			return err
		}

		if _, err := os.Stat(devpath); err == nil {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
		loops -= 1
	}
	// if we testing in the docker container, we should create devices ourselves
	if vtpm.major != 0 && vtpm.minor != 0 {
		fileMode := 0o666 | unix.S_IFCHR
		dev := unix.Mkdev(vtpm.major, vtpm.minor)
		if err := unix.Mknod(devpath, uint32(fileMode), int(dev)); err != nil {
			return &os.PathError{Op: "mknod", Path: devpath, Err: err}
		}
		return nil
	}
	return fmt.Errorf("TPM device %s did not appear", devpath)
}

func (vtpm *VTPM) GetDeviceNum() error {
	if fileInfo, err := os.Lstat(vtpm.GetTPMDevpath()); err == nil {
		if stat_t, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			devNumber := stat_t.Rdev
			vtpm.major = unix.Major(devNumber)
			vtpm.minor = unix.Minor(devNumber)
		}
	} else {
		return err
	}
	logrus.Infof("device major num: %d", vtpm.major)
	logrus.Infof("device minor num: %d", vtpm.minor)
	return nil
}

// startSwtpm creates the VTPM proxy device and start the swtpm process
func (vtpm *VTPM) startSwtpm() error {
	tpm_dev_name := fmt.Sprintf("tpm%s", vtpm.VtpmName)

	err := vtpm.setupAppArmor()
	if err != nil {
		return err
	}
	err = vtpm.setupSELinux()
	if err != nil {
		return err
	}

	tpmstate := fmt.Sprintf("dir=%s", vtpm.StatePath)
	pidfile := fmt.Sprintf("file=%s", vtpm.getPidFile())
	logfile := fmt.Sprintf("file=%s", vtpm.getLogFile())

	flags := "not-need-init"
	if hasCapability(vtpm.swtpmCaps, "flags-opt-startup") {
		flags += ",startup-clear"
	}
	args := []string{
		"--tpmstate", tpmstate,
		"-n", tpm_dev_name, "--pid", pidfile, "--log", logfile,
		"--flags", flags,
		"--locality", "reject-locality-4,allow-set-locality"}

	if vtpm.major != 0 {
		args = append(args, fmt.Sprintf("--maj=%d", vtpm.major))
	}

	if vtpm.minor != 0 {
		args = append(args, fmt.Sprintf("--min=%d", vtpm.minor))
	}

	cmd := exec.Command("swtpm_cuse", args...)
	if vtpm.Vtpmversion == VTPM_VERSION_2 {
		cmd.Args = append(cmd.Args, "--tpm2")
	}

	if len(vtpm.encryptionPassword) > 0 {
		piper, err := vtpm.setupPasswordPipe(vtpm.encryptionPassword)
		if err != nil {
			return err
		}
		cmd.ExtraFiles = append(cmd.ExtraFiles, piper)
		cmd.Args = append(cmd.Args, "--key",
			fmt.Sprintf("pwdfd=%d,mode=aes-256-cbc,kdf=pbkdf2", 3+len(cmd.ExtraFiles)-1))
		defer piper.Close()
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("swtpm failed on fd %s: %s\nlog: %s", tpm_dev_name, string(output), vtpm.ReadLog())
	}
	if vtpm.passwordPipeError != nil {
		return fmt.Errorf("Error transferring password using pipe: %v", vtpm.passwordPipeError)
	}

	vtpm.Pid, err = vtpm.waitForPidFile(10)
	if err != nil {
		return err
	}

	err = vtpm.waitForTPMDevice(50)
	if err != nil {
		return err
	}

	vtpm.resetSELinux()
	vtpm.resetAppArmor()

	return nil
}

// runSwtpmBios runs swtpm_bios to initialize the TPM
func (vtpm *VTPM) runSwtpmBios() error {
	tpmname := vtpm.GetTPMDevpath()

	cmd := exec.Command("swtpm_bios", "-n", "-cs", "-u", "--tpm-device", tpmname)
	if vtpm.Vtpmversion == VTPM_VERSION_2 {
		cmd.Args = append(cmd.Args, "--tpm2")
	} else {
		// make sure the TPM 1.2 is activated
		cmd.Args = append(cmd.Args, "-ea")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("swtpm_bios failed: %s", output)
	}
	return nil
}

// Start starts the vTPM (swtpm)
//
//   - ensure any still running vTPM, which wrote its PID into a file in its state path, is terminated
//     the swtpm will, upon normal termination, remove its PID file
//   - setup the state path
//   - if the state path was created ( = swtpm runs for the first time) also create the certificates
//   - create the device pair
//   - start the swtpm process
//   - run swtpm_bios on it to initialize the vTPM as firmware would
//   - if return code is 129, restart the vTPM to activate it and run swtpm_bios again
//
// After this method ran successfully, the TPM device (/dev/tpm%d) is available for use
func (vtpm *VTPM) Start() (bool, error) {

	vtpm.Stop(false)

	createdStatePath, err := vtpm.createStatePath()
	if err != nil {
		return false, err
	}
	defer func() {
		if err != nil {
			vtpm.Stop(createdStatePath)
		}
	}()

	err = vtpm.chownStatePath()
	if err != nil {
		return false, err
	}

	err = vtpm.startSwtpm()
	if err != nil {
		return false, err
	}

	err = vtpm.runSwtpmBios()
	if err != nil {
		return false, err
	}

	return createdStatePath, nil
}

// Stop stops a running vTPM; this method can be called at any time also
// to do partial cleanups; After this method ran, Start() can be called again
func (vtpm *VTPM) Stop(deleteStatePath bool) error {

	err := vtpm.stopByPidFile()

	vtpm.teardownSELinux()
	vtpm.teardownAppArmor()

	if deleteStatePath {
		vtpm.DeleteStatePath()
	}

	return err
}

// Get the TPM device name; this method can be called after successful Start()
func (vtpm *VTPM) GetTPMDevpath() string {
	return fmt.Sprintf("/dev/tpm%s", vtpm.VtpmName)
}

// Get the major and minor numbers of the created TPM device;
// This method can be called after successful Start()
func (vtpm *VTPM) GetMajorMinor() (uint32, uint32) {
	return vtpm.major, vtpm.minor
}

// ReadLog reads the vTPM's log file and returns the contents as a string
// This method can be called after Start()
func (vtpm *VTPM) ReadLog() string {
	output, err := ioutil.ReadFile(vtpm.getLogFile())
	if err != nil {
		return ""
	}
	return string(output)
}

// setupAppArmor creates an apparmor profile for swtpm if AppArmor is enabled and
// compiles it using apparmor_parser -r <filename> and activates it for the next
// exec.
func (vtpm *VTPM) setupAppArmor() error {
	var statefilepattern string
	var tmpStateFilePattern string

	if !apparmor.IsEnabled() {
		return nil
	}

	profilename := fmt.Sprintf("runc_%d_swtpm_tpm%s", os.Getpid(), vtpm.VtpmName)
	if vtpm.Vtpmversion == VTPM_VERSION_1_2 {
		statefilepattern = path.Join(vtpm.StatePath, "tpm-00.*")
	} else {
		statefilepattern = path.Join(vtpm.StatePath, "tpm2-00.*")
	}

	// We do not set backup as option to tpmstate dir, tmpfile (TMP{2}.*) will be used as backup.
	// Link to SWTPM_NVRAM_GetFilenameForName function: https://github.com/stefanberger/swtpm/blob/master/src/swtpm/swtpm_nvstore.c#L273
	if vtpm.Vtpmversion == VTPM_VERSION_1_2 {
		tmpStateFilePattern = path.Join(vtpm.StatePath, "TMP-00.*")
	} else {
		tmpStateFilePattern = path.Join(vtpm.StatePath, "TMP2-00.*")
	}

	profile := fmt.Sprintf("\n#include <tunables/global>\n"+
		"profile %s {\n"+
		"  #include <abstractions/base>\n"+
		"  capability setgid,\n"+
		"  capability setuid,\n"+
		"  capability sys_nice,\n"+
		"  /dev/tpm[0-9]* rw,\n"+
		"  owner /etc/group r,\n"+
		"  owner /etc/nsswitch.conf r,\n"+
		"  owner /etc/passwd r,\n"+
		"  /dev/cuse rw,\n"+
		"  %s/ rw,\n"+
		"  %s/.lock wk,\n"+
		"  %s w,\n"+
		"  %s rw,\n"+
		"  %s rw,\n"+
		"  %s rw,\n"+
		"}\n",
		profilename,
		vtpm.StatePath,
		vtpm.StatePath,
		vtpm.getLogFile(),
		vtpm.getPidFile(),
		statefilepattern,
		tmpStateFilePattern)

	vtpm.aaprofile = path.Join(vtpm.StatePath, "swtpm.apparmor")

	err := ioutil.WriteFile(vtpm.aaprofile, []byte(profile), 0600)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			vtpm.teardownAppArmor()
		}
	}()

	cmd := exec.Command("/sbin/apparmor_parser", "-r", vtpm.aaprofile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apparmor_parser -r failed: %s", string(output))
	}

	err = apparmor.ApplyProfile(profilename)
	if err != nil {
		return err
	}

	return nil
}

func (vtpm *VTPM) resetAppArmor() {
	apparmor.ApplyProfile("unconfined")
}

// teardownAppArmor removes the AppArmor profile from the system and ensures
// that the next time the process exec's no swtpm related profile is applied
func (vtpm *VTPM) teardownAppArmor() {
	vtpm.resetAppArmor()
	if len(vtpm.aaprofile) > 0 {
		cmd := exec.Command("/sbin/apparmor_parser", "-R", vtpm.aaprofile)
		cmd.Run()
		os.Remove(vtpm.aaprofile)
		vtpm.aaprofile = ""
	}
}

// setupSELinux labels the swtpm files with SELinux labels if SELinux is enabled
func (vtpm *VTPM) setupSELinux() error {
	if !selinux.GetEnabled() {
		return nil
	}

	processLabel, fileLabel := selinux.ContainerLabels()
	if len(processLabel) == 0 || len(fileLabel) == 0 {
		return nil
	}

	err := filepath.Walk(vtpm.StatePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != vtpm.StatePath {
			return filepath.SkipDir
		}
		return selinux.SetFileLabel(path, fileLabel)
	})

	err = selinux.SetFSCreateLabel(fileLabel)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile("/sys/fs/selinux/context", []byte(processLabel), 0000)
	if err != nil {
		return err
	}
	err = selinux.SetExecLabel(processLabel)
	if err != nil {
		return err
	}

	return nil
}

// resetSELinux resets the prepared SELinux labels
func (vtpm *VTPM) resetSELinux() {
	selinux.SetExecLabel("")
	selinux.SetFSCreateLabel("")
	ioutil.WriteFile("/sys/fs/selinux/context", []byte(""), 0000)
}

// teardownSELinux cleans up SELinux for next spawned process
func (vtpm *VTPM) teardownSELinux() {
	vtpm.resetSELinux()
}

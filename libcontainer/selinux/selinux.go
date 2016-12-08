// +build linux

package selinux

import (
	"bufio"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/opencontainers/runc/libcontainer/system"
)

const (
	Enforcing        = 1
	Permissive       = 0
	Disabled         = -1
	selinuxDir       = "/etc/selinux/"
	selinuxConfig    = selinuxDir + "config"
	selinuxTypeTag   = "SELINUXTYPE"
	selinuxTag       = "SELINUX"
	selinuxPath      = "/sys/fs/selinux"
	xattrNameSelinux = "security.selinux"
	stRdOnly         = 0x01
)

type selinuxState struct {
	enabled     bool
	enabledHost bool
	selinuxfs   string
	mcsList     map[string]bool
	sync.Mutex
}

var (
	once        sync.Once
	assignRegex = regexp.MustCompile(`^([^=]+)=(.*)$`)
	state       = selinuxState{
		mcsList: make(map[string]bool),
	}
)

type SELinuxContext map[string]string

// selinuxInit needs to be run once before using state.enabled* variables
func (s *selinuxState) selinuxInit() {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return
	}
	defer f.Close()

	var selinuxfs string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		txt := scanner.Text()
		// Safe as mountinfo encodes mountpoints with spaces as \040.
		sepIdx := strings.Index(txt, " - ")
		if sepIdx == -1 {
			continue
		}
		if !strings.Contains(txt[sepIdx:], "selinuxfs") {
			continue
		}
		fields := strings.Split(txt, " ")
		if len(fields) < 5 {
			continue
		}
		selinuxfs = fields[4]
		break
	}

	if selinuxfs != "" {
		var buf syscall.Statfs_t
		syscall.Statfs(selinuxfs, &buf)
		if (buf.Flags & stRdOnly) == 1 {
			selinuxfs = ""
		}
	}

	s.Lock()
	defer s.Unlock()
	s.selinuxfs = selinuxfs
	if fs := s.selinuxfs; fs != "" {
		if con, _ := Getcon(); con != "kernel" {
			s.enabled = true
			s.enabledHost = true
		}
	}
}

func (s *selinuxState) setEnable(enabled bool) bool {
	once.Do(s.selinuxInit)
	s.Lock()
	defer s.Unlock()
	s.enabled = enabled
	return s.enabled
}

func (s *selinuxState) getEnabledHost() bool {
	once.Do(s.selinuxInit)
	s.Lock()
	defer s.Unlock()
	return s.enabledHost
}

func (s *selinuxState) getEnabled() bool {
	once.Do(s.selinuxInit)
	s.Lock()
	defer s.Unlock()
	return s.enabled
}

func (s *selinuxState) getSELinuxfs() string {
	once.Do(s.selinuxInit)
	s.Lock()
	defer s.Unlock()
	return s.selinuxfs
}

// SetEnabled disables selinux support for the package
func SetEnabled() {
	state.setEnable(true)
}

// SetDisabled disables selinux support for the package
func SetDisabled() {
	state.setEnable(false)
}

// getSelinuxMountPoint returns the path to the mountpoint of an selinuxfs
// filesystem or an empty string if no mountpoint is found.  Selinuxfs is
// a proc-like pseudo-filesystem that exposes the selinux policy API to
// processes.  The existence of an selinuxfs mount is used to determine
// whether selinux is currently enabled or not.
func getSelinuxMountPoint() string {
	return state.getSELinuxfs()
}

// SelinuxEnabledHost returns whether selinux is currently enabled on the host.
func SelinuxEnabledHost() bool {
	return state.getEnabledHost()
}

// SelinuxEnabled returns whether selinux is currently enabled for containers.
func SelinuxEnabled() bool {
	return state.getEnabled()
}

func readConfig(target string) (value string) {
	var (
		val, key string
		bufin    *bufio.Reader
	)

	in, err := os.Open(selinuxConfig)
	if err != nil {
		return ""
	}
	defer in.Close()

	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err != io.EOF {
				return ""
			}
			done = true
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		if groups := assignRegex.FindStringSubmatch(line); groups != nil {
			key, val = strings.TrimSpace(groups[1]), strings.TrimSpace(groups[2])
			if key == target {
				return strings.Trim(val, "\"")
			}
		}
	}
	return ""
}

func getSELinuxPolicyRoot() string {
	return selinuxDir + readConfig(selinuxTypeTag)
}

func readCon(name string) (string, error) {
	var val string

	in, err := os.Open(name)
	if err != nil {
		return "", err
	}
	defer in.Close()

	_, err = fmt.Fscanf(in, "%s", &val)
	return val, err
}

// Setfilecon sets the SELinux label for this path or returns an error.
func Setfilecon(path string, scon string) error {
	return system.Lsetxattr(path, xattrNameSelinux, []byte(scon), 0)
}

// Getfilecon returns the SELinux label for this path or returns an error.
func Getfilecon(path string) (string, error) {
	con, err := system.Lgetxattr(path, xattrNameSelinux)
	if err != nil {
		return "", err
	}
	// Trim the NUL byte at the end of the byte buffer, if present.
	if len(con) > 0 && con[len(con)-1] == '\x00' {
		con = con[:len(con)-1]
	}
	return string(con), nil
}

func Setfscreatecon(scon string) error {
	return writeCon(fmt.Sprintf("/proc/self/task/%d/attr/fscreate", syscall.Gettid()), scon)
}

func Getfscreatecon() (string, error) {
	return readCon(fmt.Sprintf("/proc/self/task/%d/attr/fscreate", syscall.Gettid()))
}

// Getcon returns the SELinux label of the current process thread, or an error.
func Getcon() (string, error) {
	return readCon(fmt.Sprintf("/proc/self/task/%d/attr/current", syscall.Gettid()))
}

// Getpidcon returns the SELinux label of the given pid, or an error.
func Getpidcon(pid int) (string, error) {
	return readCon(fmt.Sprintf("/proc/%d/attr/current", pid))
}

func Getexeccon() (string, error) {
	return readCon(fmt.Sprintf("/proc/self/task/%d/attr/exec", syscall.Gettid()))
}

func writeCon(name string, val string) error {
	out, err := os.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer out.Close()

	if val != "" {
		_, err = out.Write([]byte(val))
	} else {
		_, err = out.Write(nil)
	}
	return err
}

func Setexeccon(scon string) error {
	return writeCon(fmt.Sprintf("/proc/self/task/%d/attr/exec", syscall.Gettid()), scon)
}

func (c SELinuxContext) Get() string {
	return fmt.Sprintf("%s:%s:%s:%s", c["user"], c["role"], c["type"], c["level"])
}

func NewContext(scon string) SELinuxContext {
	c := make(SELinuxContext)

	if len(scon) != 0 {
		con := strings.SplitN(scon, ":", 4)
		c["user"] = con[0]
		c["role"] = con[1]
		c["type"] = con[2]
		c["level"] = con[3]
	}
	return c
}

func ReserveLabel(scon string) {
	if len(scon) != 0 {
		con := strings.SplitN(scon, ":", 4)
		mcsAdd(con[3])
	}
}

func selinuxEnforcePath() string {
	return fmt.Sprintf("%s/enforce", selinuxPath)
}

func SelinuxGetEnforce() int {
	var enforce int

	enforceS, err := readCon(selinuxEnforcePath())
	if err != nil {
		return -1
	}

	enforce, err = strconv.Atoi(string(enforceS))
	if err != nil {
		return -1
	}
	return enforce
}

func SelinuxSetEnforce(mode int) error {
	return writeCon(selinuxEnforcePath(), fmt.Sprintf("%d", mode))
}

func SelinuxGetEnforceMode() int {
	switch readConfig(selinuxTag) {
	case "enforcing":
		return Enforcing
	case "permissive":
		return Permissive
	}
	return Disabled
}

func mcsAdd(mcs string) error {
	state.Lock()
	defer state.Unlock()
	if state.mcsList[mcs] {
		return fmt.Errorf("MCS Label already exists")
	}
	state.mcsList[mcs] = true
	return nil
}

func mcsDelete(mcs string) {
	state.Lock()
	defer state.Unlock()
	state.mcsList[mcs] = false
}

func IntToMcs(id int, catRange uint32) string {
	var (
		SETSIZE = int(catRange)
		TIER    = SETSIZE
		ORD     = id
	)

	if id < 1 || id > 523776 {
		return ""
	}

	for ORD > TIER {
		ORD = ORD - TIER
		TIER--
	}
	TIER = SETSIZE - TIER
	ORD = ORD + TIER
	return fmt.Sprintf("s0:c%d,c%d", TIER, ORD)
}

func uniqMcs(catRange uint32) string {
	var (
		n      uint32
		c1, c2 uint32
		mcs    string
	)

	for {
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c1 = n % catRange
		binary.Read(rand.Reader, binary.LittleEndian, &n)
		c2 = n % catRange
		if c1 == c2 {
			continue
		} else {
			if c1 > c2 {
				t := c1
				c1 = c2
				c2 = t
			}
		}
		mcs = fmt.Sprintf("s0:c%d,c%d", c1, c2)
		if err := mcsAdd(mcs); err != nil {
			continue
		}
		break
	}
	return mcs
}

func FreeLxcContexts(scon string) {
	if len(scon) != 0 {
		con := strings.SplitN(scon, ":", 4)
		mcsDelete(con[3])
	}
}

var roFileLabel string

func GetROFileLabel() (fileLabel string) {
	return roFileLabel
}

func getDefaultLabels() (processLabel string, fileLabel string) {
	var (
		val, key string
		bufin    *bufio.Reader
	)

	lxcPath := fmt.Sprintf("%s/contexts/lxc_contexts", getSELinuxPolicyRoot())
	in, err := os.Open(lxcPath)
	if err != nil {
		return "", ""
	}
	defer in.Close()

	bufin = bufio.NewReader(in)

	for done := false; !done; {
		var line string
		if line, err = bufin.ReadString('\n'); err != nil {
			if err == io.EOF {
				done = true
			} else {
				goto exit
			}
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			// Skip blank lines
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			// Skip comments
			continue
		}
		if groups := assignRegex.FindStringSubmatch(line); groups != nil {
			key, val = strings.TrimSpace(groups[1]), strings.TrimSpace(groups[2])
			if key == "process" {
				processLabel = strings.Trim(val, "\"")
			}
			if key == "file" {
				scon := NewContext(strings.Trim(val, "\""))
				// Default s0:c1023, non priv container can't use this level
				scon["level"] = "s0:c1023"
				fileLabel = scon.Get()
			}
			if key == "ro_file" {
				roFileLabel = strings.Trim(val, "\"")
			}
		}
	}
exit:
	if roFileLabel == "" {
		roFileLabel = fileLabel
	}
	return processLabel, fileLabel
}

func GetLxcContexts() (processLabel string, fileLabel string) {
	if !state.getEnabledHost() {
		return "", ""
	}

	defProcessLabel, defFileLabel := getDefaultLabels()
	if !state.getEnabled() {
		return "", defFileLabel
	}

	// 1024 Categories available, 0-1023. Only using 1023 for confinement
	// Reserving last category, s0:c1023, for privileged containers
	// Blocks confined containers from useing priv container's content
	mcs := uniqMcs(1023)
	scon := NewContext(defProcessLabel)
	scon["level"] = mcs
	processLabel = scon.Get()
	scon = NewContext(defFileLabel)
	scon["level"] = mcs
	fileLabel = scon.Get()
	return processLabel, fileLabel
}

func SecurityCheckContext(val string) error {
	return writeCon(fmt.Sprintf("%s.context", selinuxPath), val)
}

func CopyLevel(src, dest string) (string, error) {
	if src == "" {
		return "", nil
	}
	if err := SecurityCheckContext(src); err != nil {
		return "", err
	}
	if err := SecurityCheckContext(dest); err != nil {
		return "", err
	}
	scon := NewContext(src)
	tcon := NewContext(dest)
	mcsDelete(tcon["level"])
	mcsAdd(scon["level"])
	tcon["level"] = scon["level"]
	return tcon.Get(), nil
}

// Prevent users from relabing system files
func badPrefix(fpath string) error {
	var badprefixes = []string{"/usr"}

	for _, prefix := range badprefixes {
		if fpath == prefix || strings.HasPrefix(fpath, fmt.Sprintf("%s/", prefix)) {
			return fmt.Errorf("Relabeling content in %s is not allowed.", prefix)
		}
	}
	return nil
}

// Chcon changes the fpath file object to the SELinux label scon.
// If the fpath is a directory and recurse is true Chcon will walk the
// directory tree setting the label
func Chcon(fpath string, scon string, recurse bool) error {
	if scon == "" {
		return nil
	}
	if err := badPrefix(fpath); err != nil {
		return err
	}
	callback := func(p string, info os.FileInfo, err error) error {
		return Setfilecon(p, scon)
	}

	if recurse {
		return filepath.Walk(fpath, callback)
	}

	return Setfilecon(fpath, scon)
}

// DupSecOpt takes an SELinux process label and returns security options that
// can will set the SELinux Type and Level for future container processes
func DupSecOpt(src string) []string {
	if src == "" {
		return nil
	}
	con := NewContext(src)
	if con["user"] == "" ||
		con["role"] == "" ||
		con["type"] == "" ||
		con["level"] == "" {
		return nil
	}
	return []string{"label=user:" + con["user"],
		"label=role:" + con["role"],
		"label=type:" + con["type"],
		"label=level:" + con["level"]}
}

// DisableSecOpt returns a security opt that can be used to disabling SELinux
// labeling support for future container processes
func DisableSecOpt() []string {
	return []string{"label=disable"}
}

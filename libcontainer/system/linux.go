//go:build linux

package system

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unsafe"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// SetgroupsPolicy is used for setgroups policies.
type SetgroupsPolicy int

const (
	SetgroupsDefault SetgroupsPolicy = iota + 1
	SetgroupsAllow
	SetgroupsDeny
)

type ParentDeathSignal int

func (p ParentDeathSignal) Restore() error {
	if p == 0 {
		return nil
	}
	current, err := GetParentDeathSignal()
	if err != nil {
		return err
	}
	if p == current {
		return nil
	}
	return p.Set()
}

func (p ParentDeathSignal) Set() error {
	return SetParentDeathSignal(uintptr(p))
}

func Exec(cmd string, args []string, env []string) error {
	for {
		err := unix.Exec(cmd, args, env)
		if err != unix.EINTR {
			return &os.PathError{Op: "exec", Path: cmd, Err: err}
		}
	}
}

func SetParentDeathSignal(sig uintptr) error {
	if err := unix.Prctl(unix.PR_SET_PDEATHSIG, sig, 0, 0, 0); err != nil {
		return err
	}
	return nil
}

func GetParentDeathSignal() (ParentDeathSignal, error) {
	var sig int
	if err := unix.Prctl(unix.PR_GET_PDEATHSIG, uintptr(unsafe.Pointer(&sig)), 0, 0, 0); err != nil {
		return -1, err
	}
	return ParentDeathSignal(sig), nil
}

func SetKeepCaps() error {
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 1, 0, 0, 0); err != nil {
		return err
	}

	return nil
}

func ClearKeepCaps() error {
	if err := unix.Prctl(unix.PR_SET_KEEPCAPS, 0, 0, 0, 0); err != nil {
		return err
	}

	return nil
}

func Setctty() error {
	if err := unix.IoctlSetInt(0, unix.TIOCSCTTY, 0); err != nil {
		return err
	}
	return nil
}

// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, uintptr(i), 0, 0, 0)
}

// GetSubreaper returns the subreaper setting for the calling process
func GetSubreaper() (int, error) {
	var i uintptr

	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}

	return int(i), nil
}

func ExecutableMemfd(comment string, flags int) (*os.File, error) {
	// Try to use MFD_EXEC first. On pre-6.3 kernels we get -EINVAL for this
	// flag. On post-6.3 kernels, with vm.memfd_noexec=1 this ensures we get an
	// executable memfd. For vm.memfd_noexec=2 this is a bit more complicated.
	// The original vm.memfd_noexec=2 implementation incorrectly silently
	// allowed MFD_EXEC[1] -- this should be fixed in 6.6. On 6.6 and newer
	// kernels, we will get -EACCES if we try to use MFD_EXEC with
	// vm.memfd_noexec=2 (for 6.3-6.5, -EINVAL was the intended return value).
	//
	// The upshot is we only need to retry without MFD_EXEC on -EINVAL because
	// it just so happens that passing MFD_EXEC bypasses vm.memfd_noexec=2 on
	// kernels where -EINVAL is actually a security denial.
	memfd, err := unix.MemfdCreate(comment, flags|unix.MFD_EXEC)
	if err == unix.EINVAL {
		memfd, err = unix.MemfdCreate(comment, flags)
	}
	if err != nil {
		if err == unix.EACCES {
			logrus.Info("memfd_create(MFD_EXEC) failed, possibly due to vm.memfd_noexec=2 -- falling back to less secure O_TMPFILE")
		}
		err := os.NewSyscallError("memfd_create", err)
		return nil, fmt.Errorf("failed to create executable memfd: %w", err)
	}
	return os.NewFile(uintptr(memfd), "/memfd:"+comment), nil
}

// Copy is like io.Copy except it uses sendfile(2) if the source and sink are
// both (*os.File) as an optimisation to make copies faster.
func Copy(dst io.Writer, src io.Reader) (copied int64, err error) {
	dstFile, _ := dst.(*os.File)
	srcFile, _ := src.(*os.File)

	if dstFile != nil && srcFile != nil {
		fi, err := srcFile.Stat()
		if err != nil {
			goto fallback
		}
		size := fi.Size()
		for size > 0 {
			n, err := unix.Sendfile(int(dstFile.Fd()), int(srcFile.Fd()), nil, int(size))
			if n > 0 {
				size -= int64(n)
				copied += int64(n)
			}
			if err == unix.EINTR {
				continue
			}
			if err != nil {
				if copied == 0 {
					// If we haven't copied anything so far, we can safely just
					// fallback to io.Copy. We could always do the fallback but
					// it's safer to error out in the case of a partial copy
					// followed by an error (which should never happen).
					goto fallback
				}
				return copied, fmt.Errorf("partial sendfile copy: %w", err)
			}
		}
		return copied, nil
	}

fallback:
	return io.Copy(dst, src)
}

// SetLinuxPersonality sets the Linux execution personality. For more information see the personality syscall documentation.
// checkout getLinuxPersonalityFromStr() from libcontainer/specconv/spec_linux.go for type conversion.
func SetLinuxPersonality(personality int) error {
	_, _, errno := unix.Syscall(unix.SYS_PERSONALITY, uintptr(personality), 0, 0)
	if errno != 0 {
		return &os.SyscallError{Syscall: "set_personality", Err: errno}
	}
	return nil
}

// UpdateSetgroups is to set the process's setgroups policy
// This *must* be called before we touch gid_map.
func UpdateSetgroups(pid int, policy SetgroupsPolicy) error {
	var strPolicy string
	switch policy {
	case SetgroupsAllow:
		strPolicy = "allow"
	case SetgroupsDeny:
		strPolicy = "deny"
	case SetgroupsDefault:
		fallthrough
	default:
		return nil
	}
	err := os.WriteFile("/proc/"+strconv.Itoa(pid)+"/setgroups", []byte(strPolicy), 0)

	// If the kernel is too old to support /proc/pid/setgroups,
	// open(2) or write(2) will return ENOENT. This is fine.
	if errors.Is(err, unix.ENOENT) {
		return nil
	}
	return err
}

// TryMappingTool is to try to use the mapping tool to map the uid/gid.
func TryMappingTool(app string, pid int, idMappings []configs.IDMap) error {
	if app == "" {
		return fmt.Errorf("no mapping tool specified")
	}

	argv := []string{strconv.Itoa(pid)}
	for _, m := range idMappings {
		argv = append(argv, strconv.FormatInt(m.ContainerID, 10), strconv.FormatInt(m.HostID, 10), strconv.FormatInt(m.Size, 10))
	}
	cmd := exec.Command(app, argv...)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
}

// UpdateUidmap is to update the uid map of the process.
func UpdateUidmap(app string, pid int, uidMappings []configs.IDMap) error {
	if len(uidMappings) == 0 {
		return nil
	}

	data := []string{}
	for _, m := range uidMappings {
		data = append(data, m.ToString())
	}
	logrus.Debugf("update /proc/%d/uid_map to '%s'", pid, strings.Join(data, "\n"))
	err := os.WriteFile("/proc/"+strconv.Itoa(pid)+"/uid_map", []byte(strings.Join(data, "\n")), 0)
	if errors.Is(err, unix.EPERM) {
		logrus.Debugf("update /proc/%d/uid_map got -EPERM (trying %s)", pid, app)
		return TryMappingTool(app, pid, uidMappings)
	}
	return err
}

// UpdateGidmap is to update the gid map of the process.
func UpdateGidmap(app string, pid int, gidMappings []configs.IDMap) error {
	if len(gidMappings) == 0 {
		return nil
	}

	data := []string{}
	for _, m := range gidMappings {
		data = append(data, m.ToString())
	}
	logrus.Debugf("update /proc/%d/gid_map to '%s'", pid, strings.Join(data, "\n"))
	err := os.WriteFile("/proc/"+strconv.Itoa(pid)+"/gid_map", []byte(strings.Join(data, "\n")), 0)
	if errors.Is(err, unix.EPERM) {
		logrus.Debugf("update /proc/%d/gid_map got -EPERM (trying %s)", pid, app)
		return TryMappingTool(app, pid, gidMappings)
	}
	return err
}

// UpdateTimeNsOffsets is to update the time namespace offsets of the process.
func UpdateTimeNsOffsets(pid int, offsets map[string]specs.LinuxTimeOffset) error {
	if len(offsets) == 0 {
		return nil
	}
	var data []string
	for clock, offset := range offsets {
		data = append(data, clock+" "+strconv.FormatInt(offset.Secs, 10)+" "+strconv.FormatInt(int64(offset.Nanosecs), 10))
	}
	logrus.Debugf("update /proc/%d/timens_offsets to '%s'", pid, strings.Join(data, "\n"))
	return os.WriteFile("/proc/"+strconv.Itoa(pid)+"/timens_offsets", []byte(strings.Join(data, "\n")), 0)
}

// UpdateOomScoreAdj is to update oom_score_adj of the process.
func UpdateOomScoreAdj(oomScoreAdj string) error {
	if len(oomScoreAdj) == 0 {
		return nil
	}
	logrus.Debugf("update /proc/self/oom_score_adj to '%s'", oomScoreAdj)
	return os.WriteFile("/proc/self/oom_score_adj", []byte(oomScoreAdj), 0)
}

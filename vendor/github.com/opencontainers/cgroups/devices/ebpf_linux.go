package devices

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"unsafe"

	"github.com/cilium/ebpf/asm"
	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func bpf(cmd uintptr, attr unsafe.Pointer, size uintptr) (uintptr, error) {
	r1, _, err := unix.Syscall(unix.SYS_BPF, cmd, uintptr(attr), size)
	runtime.KeepAlive(attr)
	if err != 0 {
		return r1, err
	}
	return r1, nil
}

// bpfProgLoad loads a BPF_PROG_TYPE_CGROUP_DEVICE program and returns its fd.
func bpfProgLoad(insns asm.Instructions, license string) (int, error) {
	buf := bytes.NewBuffer(make([]byte, 0, insns.Size()))
	if err := insns.Marshal(buf, nativeEndian); err != nil {
		return -1, err
	}
	insnsBytes := buf.Bytes()

	licensePtr, err := unix.BytePtrFromString(license)
	if err != nil {
		return -1, err
	}

	// Subset of struct bpf_attr for BPF_PROG_LOAD. Fields past the ones we set
	// are left zero; the kernel zero-fills any part of bpf_attr beyond the size
	// we pass.
	attr := struct {
		progType uint32
		insnCnt  uint32
		insns    uint64 // pointer
		license  uint64 // pointer
		logLevel uint32
		logSize  uint32
		logBuf   uint64 // pointer
	}{
		progType: unix.BPF_PROG_TYPE_CGROUP_DEVICE,
		insnCnt:  uint32(len(insnsBytes) / asm.InstructionSize),
		insns:    uint64(uintptr(unsafe.Pointer(&insnsBytes[0]))),
		license:  uint64(uintptr(unsafe.Pointer(licensePtr))),
	}

	fd, err := bpf(unix.BPF_PROG_LOAD, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	// attr holds the pointers as integers, so the GC can't see them; keep the
	// referenced objects alive until the syscall returns.
	runtime.KeepAlive(insnsBytes)
	runtime.KeepAlive(licensePtr)
	if err == nil {
		return int(fd), nil
	}

	// The load failed. Retry with the verifier log enabled so we can include
	// it in the error (the first attempt skips it, as it is the fast path).
	log := make([]byte, 64*1024)
	attr.logLevel = 1
	attr.logSize = uint32(len(log))
	attr.logBuf = uint64(uintptr(unsafe.Pointer(&log[0])))

	fd, err = bpf(unix.BPF_PROG_LOAD, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	runtime.KeepAlive(insnsBytes)
	runtime.KeepAlive(licensePtr)
	runtime.KeepAlive(log)
	if err == nil {
		return int(fd), nil
	}
	if n := bytes.IndexByte(log, 0); n > 0 {
		return -1, fmt.Errorf("%w: %s", err, bytes.TrimRight(log[:n], "\n"))
	}
	return -1, err
}

// bpfProgGetFdByID returns the fd for the BPF program with the given ID.
func bpfProgGetFdByID(id uint32) (int, error) {
	// The kernel zero-fills the rest of bpf_attr beyond the size we pass.
	attr := struct{ id uint32 }{id}
	fd, err := bpf(unix.BPF_PROG_GET_FD_BY_ID, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	if err != nil {
		return -1, err
	}
	return int(fd), nil
}

// bpfProgAttach attaches progFd to cgroupFd with the given flags. If replaceFd
// is >= 0, its fd is set in replaceBpfFd (for BPF_F_REPLACE semantics).
func bpfProgAttach(cgroupFd, progFd int, attachFlags uint32, replaceFd int) error {
	attr := struct {
		targetFd     uint32
		attachBpfFd  uint32
		attachType   uint32
		attachFlags  uint32
		replaceBpfFd uint32
	}{
		targetFd:    uint32(cgroupFd),
		attachBpfFd: uint32(progFd),
		attachType:  uint32(unix.BPF_CGROUP_DEVICE),
		attachFlags: attachFlags,
	}
	if replaceFd >= 0 {
		attr.replaceBpfFd = uint32(replaceFd)
	}
	_, err := bpf(unix.BPF_PROG_ATTACH, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}

// bpfProgDetach detaches progFd from cgroupFd.
func bpfProgDetach(cgroupFd, progFd int) error {
	// The kernel zero-fills the rest of bpf_attr beyond the size we pass.
	attr := struct {
		targetFd    uint32
		attachBpfFd uint32
		attachType  uint32
	}{
		targetFd:    uint32(cgroupFd),
		attachBpfFd: uint32(progFd),
		attachType:  uint32(unix.BPF_CGROUP_DEVICE),
	}
	_, err := bpf(unix.BPF_PROG_DETACH, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}

func findAttachedCgroupDeviceFilters(dirFd int) (_ []int, retErr error) {
	type bpfAttrQuery struct {
		TargetFd    uint32
		AttachType  uint32
		QueryType   uint32
		AttachFlags uint32
		ProgIds     uint64 // __aligned_u64
		ProgCnt     uint32
	}

	// Currently you can only have 64 eBPF programs attached to a cgroup.
	size := 64
	retries := 0
	for retries < 10 {
		progIds := make([]uint32, size)
		query := bpfAttrQuery{
			TargetFd:   uint32(dirFd),
			AttachType: uint32(unix.BPF_CGROUP_DEVICE),
			ProgIds:    uint64(uintptr(unsafe.Pointer(&progIds[0]))),
			ProgCnt:    uint32(len(progIds)),
		}

		// Fetch the list of program ids. bpf() keeps &query alive for the
		// duration of the syscall, and query.ProgCnt is read right after.
		_, err := bpf(unix.BPF_PROG_QUERY, unsafe.Pointer(&query), unsafe.Sizeof(query))
		runtime.KeepAlive(progIds)
		size = int(query.ProgCnt)
		if err != nil {
			// On ENOSPC we get the correct number of programs.
			if errors.Is(err, unix.ENOSPC) {
				retries++
				continue
			}
			return nil, fmt.Errorf("bpf_prog_query(BPF_CGROUP_DEVICE) failed: %w", err)
		}

		// Convert the ids to program fds.
		// On error we don't return the fds slice, so close the fds stored there.
		progIds = progIds[:size]
		fds := make([]int, 0, len(progIds))
		defer func() {
			if retErr != nil {
				for _, fd := range fds {
					unix.Close(fd)
				}
			}
		}()

		for _, progId := range progIds {
			fd, err := bpfProgGetFdByID(progId)
			if err != nil {
				// We skip over programs that give us -EACCES or -EPERM. This
				// is necessary because there may be BPF programs that have
				// been attached (such as with --systemd-cgroup) which have an
				// LSM label that blocks us from interacting with the program.
				//
				// Because additional BPF_CGROUP_DEVICE programs only can add
				// restrictions, there's no real issue with just ignoring these
				// programs (and stops runc from breaking on distributions with
				// very strict SELinux policies).
				if errors.Is(err, os.ErrPermission) {
					logrus.Debugf("ignoring existing CGROUP_DEVICE program (prog_id=%v) which cannot be accessed by runc -- likely due to LSM policy: %v", progId, err)
					continue
				}
				return nil, fmt.Errorf("cannot fetch program from id: %w", err)
			}
			fds = append(fds, fd)
		}
		runtime.KeepAlive(progIds)
		return fds, nil
	}

	return nil, errors.New("could not get complete list of CGROUP_DEVICE programs")
}

var (
	haveBpfProgReplaceBool bool
	haveBpfProgReplaceOnce sync.Once
)

// Loosely based on the BPF_F_REPLACE support check in
// https://github.com/cilium/ebpf/blob/v0.6.0/link/syscalls.go.
func haveBpfProgReplace() bool {
	haveBpfProgReplaceOnce.Do(func() {
		progFd, err := bpfProgLoad(asm.Instructions{
			asm.Mov.Imm(asm.R0, 0),
			asm.Return(),
		}, "MIT")
		if err != nil {
			logrus.Warnf("checking for BPF_F_REPLACE support: bpfProgLoad failed: %v", err)
			return
		}
		defer unix.Close(progFd)

		devnull, err := os.Open("/dev/null")
		if err != nil {
			logrus.Warnf("checking for BPF_F_REPLACE support: open dummy target fd: %v", err)
			return
		}
		defer devnull.Close()

		// We know that we have BPF_PROG_ATTACH since we can load
		// BPF_CGROUP_DEVICE programs. If passing BPF_F_REPLACE gives us EINVAL
		// we know that the feature isn't present.
		//
		// We rely on the target fd being checked after attachFlags in the
		// kernel. Attempting to "replace" our BPF program with itself always
		// fails, but we should get -EINVAL if BPF_F_REPLACE is not supported,
		// and -EBADF (from the dummy target fd) if it is.
		err = bpfProgAttach(int(devnull.Fd()), progFd, unix.BPF_F_ALLOW_MULTI|unix.BPF_F_REPLACE, progFd)
		if errors.Is(err, unix.EINVAL) {
			// not supported
			return
		}
		if !errors.Is(err, unix.EBADF) {
			// If we see any new errors here, it's possible that there is a
			// regression due to a kernel update and the above EINVAL
			// checks are not working. So, be loud about it so someone notices
			// and we can get the issue fixed quicker.
			logrus.Warnf("checking for BPF_F_REPLACE: got unexpected (not EBADF or EINVAL) error: %v", err)
		}
		haveBpfProgReplaceBool = true
	})
	return haveBpfProgReplaceBool
}

// loadAttachCgroupDeviceFilter installs eBPF device filter program to /sys/fs/cgroup/<foo> directory.
//
// Requires the system to be running in cgroup2 unified-mode with kernel >= 4.15 .
//
// https://github.com/torvalds/linux/commit/ebc614f687369f9df99828572b1d85a7c2de3d92
func loadAttachCgroupDeviceFilter(insts asm.Instructions, license string, dirFd int) error {
	// Increase `ulimit -l` limit to avoid BPF_PROG_LOAD error (#2167).
	// This limit is not inherited into the container.
	memlockLimit := &unix.Rlimit{
		Cur: unix.RLIM_INFINITY,
		Max: unix.RLIM_INFINITY,
	}
	_ = unix.Setrlimit(unix.RLIMIT_MEMLOCK, memlockLimit)

	// Get the list of existing programs.
	oldFds, err := findAttachedCgroupDeviceFilters(dirFd)
	if err != nil {
		return err
	}
	defer func() {
		for _, fd := range oldFds {
			unix.Close(fd)
		}
	}()

	useReplaceProg := haveBpfProgReplace() && len(oldFds) == 1

	// Generate new program.
	progFd, err := bpfProgLoad(insts, license)
	if err != nil {
		return err
	}
	// Once the program is attached, the kernel keeps it alive via the cgroup
	// attachment, so we no longer need our own fd; we also don't need it if the
	// attach below fails. Either way, close it on return.
	defer unix.Close(progFd)

	// If there is only one old program, we can just replace it directly.
	replaceFd := -1
	attachFlags := uint32(unix.BPF_F_ALLOW_MULTI)
	if useReplaceProg {
		replaceFd = oldFds[0]
		attachFlags |= unix.BPF_F_REPLACE
	}
	err = bpfProgAttach(dirFd, progFd, attachFlags, replaceFd)
	if err != nil {
		return fmt.Errorf("failed to call BPF_PROG_ATTACH (BPF_CGROUP_DEVICE, BPF_F_ALLOW_MULTI): %w", err)
	}

	if !useReplaceProg {
		logLevel := logrus.DebugLevel
		// If there was more than one old program, give a warning (since this
		// really shouldn't happen with runc-managed cgroups) and then detach
		// all the old programs.
		if len(oldFds) > 1 {
			// NOTE: Ideally this should be a warning but it turns out that
			//       systemd-managed cgroups trigger this warning (apparently
			//       systemd doesn't delete old non-systemd programs when
			//       setting properties).
			logrus.Infof("found more than one filter (%d) attached to a cgroup -- removing extra filters!", len(oldFds))
			logLevel = logrus.InfoLevel
		}
		for idx, oldFd := range oldFds {
			logrus.WithFields(logrus.Fields{
				"fd": oldFd,
			}).Logf(logLevel, "removing old filter %d from cgroup", idx)
			err = bpfProgDetach(dirFd, oldFd)
			if err != nil {
				return fmt.Errorf("failed to call BPF_PROG_DETACH (BPF_CGROUP_DEVICE) on old filter program: %w", err)
			}
		}
	}
	return nil
}

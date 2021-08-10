package fs

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/utils"
)

// The absolute path to the root of the cgroup hierarchies.
var (
	cgroupRootLock sync.Mutex
	cgroupRoot     string
)

const defaultCgroupRoot = "/sys/fs/cgroup"

func tryDefaultCgroupRoot() string {
	var st, pst unix.Stat_t

	// (1) it should be a directory...
	err := unix.Lstat(defaultCgroupRoot, &st)
	if err != nil || st.Mode&unix.S_IFDIR == 0 {
		return ""
	}

	// (2) ... and a mount point ...
	err = unix.Lstat(filepath.Dir(defaultCgroupRoot), &pst)
	if err != nil {
		return ""
	}

	if st.Dev == pst.Dev {
		// parent dir has the same dev -- not a mount point
		return ""
	}

	// (3) ... of 'tmpfs' fs type.
	var fst unix.Statfs_t
	err = unix.Statfs(defaultCgroupRoot, &fst)
	if err != nil || fst.Type != unix.TMPFS_MAGIC {
		return ""
	}

	// (4) it should have at least 1 entry ...
	dir, err := os.Open(defaultCgroupRoot)
	if err != nil {
		return ""
	}
	names, err := dir.Readdirnames(1)
	if err != nil {
		return ""
	}
	if len(names) < 1 {
		return ""
	}
	// ... which is a cgroup mount point.
	err = unix.Statfs(filepath.Join(defaultCgroupRoot, names[0]), &fst)
	if err != nil || fst.Type != unix.CGROUP_SUPER_MAGIC {
		return ""
	}

	return defaultCgroupRoot
}

// Gets the cgroupRoot.
func getCgroupRoot() (string, error) {
	cgroupRootLock.Lock()
	defer cgroupRootLock.Unlock()

	if cgroupRoot != "" {
		return cgroupRoot, nil
	}

	// fast path
	cgroupRoot = tryDefaultCgroupRoot()
	if cgroupRoot != "" {
		return cgroupRoot, nil
	}

	// slow path: parse mountinfo
	mi, err := cgroups.GetCgroupMounts(false)
	if err != nil {
		return "", err
	}
	if len(mi) < 1 {
		return "", errors.New("no cgroup mount found in mountinfo")
	}

	// Get the first cgroup mount (e.g. "/sys/fs/cgroup/memory"),
	// use its parent directory.
	root := filepath.Dir(mi[0].Mountpoint)

	if _, err := os.Stat(root); err != nil {
		return "", err
	}

	cgroupRoot = root
	return cgroupRoot, nil
}

type cgroupData struct {
	root      string
	innerPath string
	config    *configs.Cgroup
	pid       int
}

func getCgroupData(c *configs.Cgroup, pid int) (*cgroupData, error) {
	root, err := getCgroupRoot()
	if err != nil {
		return nil, err
	}

	if (c.Name != "" || c.Parent != "") && c.Path != "" {
		return nil, errors.New("cgroup: either Path or Name and Parent should be used")
	}

	// XXX: Do not remove this code. Path safety is important! -- cyphar
	cgPath := utils.CleanPath(c.Path)
	cgParent := utils.CleanPath(c.Parent)
	cgName := utils.CleanPath(c.Name)

	innerPath := cgPath
	if innerPath == "" {
		innerPath = filepath.Join(cgParent, cgName)
	}

	return &cgroupData{
		root:      root,
		innerPath: innerPath,
		config:    c,
		pid:       pid,
	}, nil
}

func (raw *cgroupData) path(subsystem string) (string, error) {
	// If the cgroup name/path is absolute do not look relative to the cgroup of the init process.
	if filepath.IsAbs(raw.innerPath) {
		mnt, err := cgroups.FindCgroupMountpoint(raw.root, subsystem)
		// If we didn't mount the subsystem, there is no point we make the path.
		if err != nil {
			return "", err
		}

		// Sometimes subsystems can be mounted together as 'cpu,cpuacct'.
		return filepath.Join(raw.root, filepath.Base(mnt), raw.innerPath), nil
	}

	// Use GetOwnCgroupPath instead of GetInitCgroupPath, because the creating
	// process could in container and shared pid namespace with host, and
	// /proc/1/cgroup could point to whole other world of cgroups.
	parentPath, err := cgroups.GetOwnCgroupPath(subsystem)
	if err != nil {
		return "", err
	}

	return filepath.Join(parentPath, raw.innerPath), nil
}

func join(path string, pid int) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	return cgroups.WriteCgroupProc(path, pid)
}

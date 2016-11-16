// +build linux

package fs

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

/*
 * About Intel RDT/CAT feature:
 * Intel platforms with new Xeon CPU support Resource Director Technology (RDT).
 * Intel Cache Allocation Technology (CAT) is a sub-feature of RDT. Currently L3
 * Cache is the only resource that is supported in RDT.
 *
 * This feature provides a way for the software to restrict cache allocation to a
 * defined 'subset' of L3 cache which may be overlapping with other 'subsets'.
 * The different subsets are identified by class of service (CLOS) and each CLOS
 * has a capacity bitmask (CBM).
 *
 * For more information about Intel RDT/CAT can be found in the section 17.17
 * of Intel Software Developer Manual.
 *
 * About Intel RDT/CAT kernel interface:
 * In Linux kernel, the interface is defined and exposed via "resource control"
 * filesystem, which is a "cgroup-like" interface.
 *
 * Comparing with cgroups, it has similar process management lifecycle and
 * interfaces in a container. But unlike cgroups' hierarchy, it has single level
 * filesystem layout.
 *
 * Intel RDT "resource control" filesystem hierarchy:
 * mount -t resctrl resctrl /sys/fs/resctrl
 * tree /sys/fs/resctrl
 * /sys/fs/resctrl/
 * |-- info
 * |   |-- L3
 * |       |-- cbm_mask
 * |       |-- num_closids
 * |-- cpus
 * |-- schemata
 * |-- tasks
 * |-- <container_id>
 *     |-- cpus
 *     |-- schemata
 *     |-- tasks
 *
 * For runc, we can make use of `tasks` and `schemata` configuration for L3 cache
 * resource constraints.
 *
 *  The file `tasks` has a list of tasks that belongs to this group (e.g.,
 * <container_id>" group). Tasks can be added to a group by writing the task ID
 * to the "tasks" file  (which will automatically remove them from the previous
 * group to which they belonged). New tasks created by fork(2) and clone(2) are
 * added to the same group as their parent. If a pid is not in any sub group, it is
 * in root group.
 *
 * The file `schemata` has allocation bitmasks/values for L3 cache on each socket,
 * which contains L3 cache id and capacity bitmask (CBM).
 * 	Format: "L3:<cache_id0>=<cbm0>;<cache_id1>=<cbm1>;..."
 * For example, on a two-socket machine, L3's schema line could be `L3:0=ff;1=c0`
 * which means L3 cache id 0's CBM is 0xff, and L3 cache id 1's CBM is 0xc0.
 *
 * The valid L3 cache CBM is a *contiguous bits set* and number of bits that can
 * be set is less than the max bit. The max bits in the CBM is varied among
 * supported Intel Xeon platforms. In Intel RDT "resource control" filesystem
 * layout, the CBM in a group should be a subset of the CBM in root. Kernel will
 * check if it is valid when writing. e.g., 0xfffff in root indicates the max bits
 * of CBM is 20 bits, which mapping to entire L3 cache capacity. Some valid CBM
 * values to set in a group: 0xf, 0xf0, 0x3ff, 0x1f00 and etc.
 *
 * For more information about Intel RDT/CAT kernel interface:
 * https://git.kernel.org/cgit/linux/kernel/git/tip/tip.git/commit/?h=x86/cache&id=f20e57892806ad244eaec7a7ae365e78fee53377
 *
 * An example for runc:
 * There are two L3 caches in the two-socket machine, the default CBM is 0xfffff
 * and the max CBM length is 20 bits. This configuration assigns 4/5 of L3 cache
 * id 0 and the whole L3 cache id 1 for the container:
 *
 * "linux": {
 * 	"resources": {
 * 		"intelRdt": {
 * 			"l3CacheSchema": "L3:0=ffff0;1=fffff"
 * 		}
 * 	}
 * }
 */

type IntelRdtGroup struct {
}

func (s *IntelRdtGroup) Name() string {
	return "intel_rdt"
}

func (s *IntelRdtGroup) Apply(d *cgroupData) error {
	data, err := getIntelRdtData(d.config, d.pid, d.containerId)
	if err != nil && !cgroups.IsNotFound(err) {
		return err
	}

	if _, err := data.join(data.containerId); err != nil {
		return err
	}

	return nil
}

func (s *IntelRdtGroup) Set(path string, cgroup *configs.Cgroup) error {
	// About L3 cache schemata file:
	// The schema has allocation masks/values for L3 cache on each socket,
	// which contains L3 cache id and capacity bitmask (CBM).
	//     Format: "L3:<cache_id0>=<cbm0>;<cache_id1>=<cbm1>;..."
	// For example, on a two-socket machine, L3's schema line could be:
	//     L3:0=ff;1=c0
	// Which means L3 cache id 0's CBM is 0xff, and L3 cache id 1's CBM is 0xc0.
	//
	// About L3 cache CBM validity:
	// The valid L3 cache CBM is a *contiguous bits set* and number of
	// bits that can be set is less than the max bit. The max bits in the
	// CBM is varied among supported Intel Xeon platforms. In Intel RDT
	// "resource control" filesystem layout, the CBM in a group should
	// be a subset of the CBM in root. Kernel will check if it is valid
	// when writing.
	// e.g., 0xfffff in root indicates the max bits of CBM is 20 bits,
	// which mapping to entire L3 cache capacity. Some valid CBM values
	// to set in a group: 0xf, 0xf0, 0x3ff, 0x1f00 and etc.
	l3CacheSchema := cgroup.Resources.IntelRdtL3CacheSchema
	if l3CacheSchema != "" {
		if err := writeFile(path, "schemata", l3CacheSchema+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *IntelRdtGroup) Remove(d *cgroupData) error {
	path, err := GetIntelRdtPath(d.containerId)
	if err != nil {
		return err
	}
	if err := removePath(path, nil); err != nil {
		return err
	}
	return nil
}

func (s *IntelRdtGroup) GetStats(path string, stats *cgroups.Stats) error {
	// The read-only default "schemata" in root
	rootPath, err := getIntelRdtRoot()
	if err != nil {
		return err
	}
	schemaRoot, err := getCgroupParamString(rootPath, "schemata")
	if err != nil {
		return err
	}
	stats.IntelRdtStats.IntelRdtRootStats.L3CacheSchema = schemaRoot

	// The stats in "container_id" group
	schema, err := getCgroupParamString(path, "schemata")
	if err != nil {
		return err
	}
	stats.IntelRdtStats.IntelRdtGroupStats.L3CacheSchema = schema

	return nil
}

const (
	IntelRdtTasks = "tasks"
)

var (
	ErrIntelRdtNotEnabled = errors.New("intelrdt: config provided but Intel RDT not supported")

	// The root path of the Intel RDT "resource control" filesystem
	intelRdtRoot string
)

type intelRdtData struct {
	root        string
	config      *configs.Cgroup
	pid         int
	containerId string
}

// The read-only Intel RDT related system information in root
type IntelRdtInfo struct {
	CbmMask   uint64 `json:"cbm_mask,omitempty"`
	NumClosid uint64 `json:"num_closid,omitempty"`
}

// Return the mount point path of Intel RDT "resource control" filesysem
func findIntelRdtMountpointDir() (string, error) {
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		text := s.Text()
		fields := strings.Split(text, " ")
		// Safe as mountinfo encodes mountpoints with spaces as \040.
		index := strings.Index(text, " - ")
		postSeparatorFields := strings.Fields(text[index+3:])
		numPostFields := len(postSeparatorFields)

		// This is an error as we can't detect if the mount is for "Intel RDT"
		if numPostFields == 0 {
			return "", fmt.Errorf("Found no fields post '-' in %q", text)
		}

		if postSeparatorFields[0] == "resctrl" {
			// Check that the mount is properly formated.
			if numPostFields < 3 {
				return "", fmt.Errorf("Error found less than 3 fields post '-' in %q", text)
			}

			return fields[4], nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}

	return "", err
}

// Gets the root path of Intel RDT "resource control" filesystem
func getIntelRdtRoot() (string, error) {
	if intelRdtRoot != "" {
		return intelRdtRoot, nil
	}

	root, err := findIntelRdtMountpointDir()
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(root); err != nil {
		return "", err
	}

	intelRdtRoot = root
	return intelRdtRoot, nil
}

func getIntelRdtData(c *configs.Cgroup, pid int, containerId string) (*intelRdtData, error) {
	rootPath, err := getIntelRdtRoot()
	if err != nil {
		return nil, err
	}
	return &intelRdtData{
		root:        rootPath,
		config:      c,
		pid:         pid,
		containerId: containerId,
	}, nil
}

// WriteIntelRdtTasks writes the specified pid into the "tasks" file
func WriteIntelRdtTasks(dir string, pid int) error {
	if dir == "" {
		return fmt.Errorf("no such directory for %s", IntelRdtTasks)
	}

	// Dont attach any pid if -1 is specified as a pid
	if pid != -1 {
		if err := ioutil.WriteFile(filepath.Join(dir, IntelRdtTasks), []byte(strconv.Itoa(pid)), 0700); err != nil {
			return fmt.Errorf("failed to write %v to %v: %v", pid, IntelRdtTasks, err)
		}
	}
	return nil
}

func (raw *intelRdtData) join(name string) (string, error) {
	path := filepath.Join(raw.root, name)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", err
	}

	if err := WriteIntelRdtTasks(path, raw.pid); err != nil {
		return "", err
	}
	return path, nil
}

func isIntelRdtMounted() bool {
	_, err := getIntelRdtRoot()
	if err != nil {
		if !cgroups.IsNotFound(err) {
			return false
		}

		// If not mounted, we try to mount again:
		// mount -t resctrl resctrl /sys/fs/resctrl
		if err := os.MkdirAll("/sys/fs/resctrl", 0755); err != nil {
			return false
		}
		if err := exec.Command("mount", "-t", "resctrl", "resctrl", "/sys/fs/resctrl").Run(); err != nil {
			return false
		}
	}

	return true
}

func parseCpuInfoFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return false, err
		}

		text := s.Text()
		flags := strings.Split(text, " ")

		for _, flag := range flags {
			if flag == "rdt_a" {
				return true, nil
			}
		}
	}
	return false, nil
}

// Check if Intel RDT is enabled
func IsIntelRdtEnabled() bool {
	// 1. check if hardware and kernel support Intel RDT feature
	// "rdt" flag is set if supported
	isFlagSet, err := parseCpuInfoFile("/proc/cpuinfo")
	if err != nil {
		return false
	}

	// 2. check if Intel RDT "resource control" filesystem is mounted
	isMounted := isIntelRdtMounted()

	return isFlagSet && isMounted
}

// Get Intel RDT "resource control" filesystem path
func GetIntelRdtPath(id string) (string, error) {
	rootPath, err := getIntelRdtRoot()
	if err != nil {
		return "", err
	}

	path := filepath.Join(rootPath, id)
	return path, nil
}

// Get read-only Intel RDT related system information
func GetIntelRdtInfo() (*IntelRdtInfo, error) {
	intelRdtInfo := &IntelRdtInfo{}

	rootPath, err := getIntelRdtRoot()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(rootPath, "info", "l3")
	cbmMask, err := getCgroupParamUint(path, "cbm_mask")
	if err != nil {
		return nil, err
	}
	numClosid, err := getCgroupParamUint(path, "num_closid")
	if err != nil {
		return nil, err
	}

	intelRdtInfo.CbmMask = cbmMask
	intelRdtInfo.NumClosid = numClosid

	return intelRdtInfo, nil
}

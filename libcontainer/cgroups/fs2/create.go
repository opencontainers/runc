package fs2

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
)

// neededControllers returns the string to write to cgroup.subtree_control,
// containing the list of controllers to enable (for example, "+cpu +pids"),
// based on (1) controllers available and (2) resources that are being set.
//
// The resulting string does not include "pseudo" controllers such as
// "freezer" and "devices".
func neededControllers(cgroup *configs.Cgroup) ([]string, error) {
	var list []string

	if cgroup == nil {
		return list, nil
	}

	// list of all available controllers
	const file = UnifiedMountpoint + "/cgroup.controllers"
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return list, err
	}
	avail := make(map[string]struct{})
	for _, ctr := range strings.Fields(string(content)) {
		avail[ctr] = struct{}{}
	}

	// add the controller if available
	add := func(controller string) {
		if _, ok := avail[controller]; ok {
			list = append(list, "+"+controller)
		}
	}

	if isPidsSet(cgroup) {
		add("pids")
	}
	if isMemorySet(cgroup) {
		add("memory")
	}
	if isIoSet(cgroup) {
		add("io")
	}
	if isCpuSet(cgroup) {
		add("cpu")
	}
	if isCpusetSet(cgroup) {
		add("cpuset")
	}
	if isHugeTlbSet(cgroup) {
		add("hugetlb")
	}

	return list, nil
}

// CreateCgroupPath creates cgroupv2 path, enabling all the
// needed controllers in the process.
func CreateCgroupPath(path string, c *configs.Cgroup) (Err error) {
	if !strings.HasPrefix(path, UnifiedMountpoint) {
		return fmt.Errorf("invalid cgroup path %s", path)
	}

	ctrs, err := neededControllers(c)
	if err != nil {
		return err
	}
	allCtrs := strings.Join(ctrs, " ")

	elements := strings.Split(path, "/")
	elements = elements[3:]
	current := "/sys/fs"
	for i, e := range elements {
		current = filepath.Join(current, e)
		if i > 0 {
			if err := os.Mkdir(current, 0755); err != nil {
				if !os.IsExist(err) {
					return err
				}
			} else {
				// If the directory was created, be sure it is not left around on errors.
				current := current
				defer func() {
					if Err != nil {
						os.Remove(current)
					}
				}()
			}
			// Write cgroup.type explicitly.
			// Otherwise ENOTSUP may happen.
			cgType := filepath.Join(current, "cgroup.type")
			_ = ioutil.WriteFile(cgType, []byte("threaded"), 0644)
		}
		// enable needed controllers
		if i < len(elements)-1 {
			file := filepath.Join(current, "cgroup.subtree_control")
			if err := ioutil.WriteFile(file, []byte(allCtrs), 0644); err != nil {
				// try write one by one
				for _, ctr := range ctrs {
					_ = ioutil.WriteFile(file, []byte(ctr), 0644)
				}
			}
			// Some controllers might not be enabled when rootless or containerized,
			// but we don't catch the error here. (Caught in setXXX() functions.)
		}
	}

	return nil
}

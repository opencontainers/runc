package fs2

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// CreateCgroupPath creates cgroupv2 path, enabling all the
// available controllers in the process.
func CreateCgroupPath(path string) (Err error) {
	const file = UnifiedMountpoint + "/cgroup.controllers"
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	ctrs := bytes.Fields(content)
	res := append([]byte("+"), bytes.Join(ctrs, []byte(" +"))...)

	current := "/sys/fs"
	elements := strings.Split(path, "/")
	for i, e := range elements[3:] {
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
		}
		if i < len(elements[3:])-1 {
			if err := ioutil.WriteFile(filepath.Join(current, "cgroup.subtree_control"), res, 0755); err != nil {
				return err
			}
		}
	}
	return nil
}

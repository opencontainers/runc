// +build linux

/*
Utility for testing cgroup operations.

Creates a mock of the cgroup filesystem for the duration of the test.
*/
package fs

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	"github.com/opencontainers/runc/libcontainer/configs"
)

func init() {
	cgroups.TestMode = true
}

type CgroupTestUtil struct {
	// CgroupData holds cgroup data to use in tests.
	CgroupData *cgroupData

	// CgroupPath is the path to the mock cgroup directory.
	CgroupPath string

	// tempDir is the temporary directory to store mock cgroup filesystem.
	tempDir string
	t       *testing.T
}

// newCgroupTestUtil creates a new test util for the specified subsystem.
func newCgroupTestUtil(subsystem string, t *testing.T) *CgroupTestUtil {
	d := &cgroupData{
		config: &configs.Cgroup{},
	}
	d.config.Resources = &configs.Resources{}
	tempDir, err := ioutil.TempDir("", "cgroup_test")
	if err != nil {
		t.Fatal(err)
	}
	d.root = tempDir
	testCgroupPath := filepath.Join(d.root, subsystem)
	if err != nil {
		t.Fatal(err)
	}

	// Ensure the full mock cgroup path exists.
	err = os.MkdirAll(testCgroupPath, 0o755)
	if err != nil {
		t.Fatal(err)
	}
	return &CgroupTestUtil{CgroupData: d, CgroupPath: testCgroupPath, tempDir: tempDir, t: t}
}

func (c *CgroupTestUtil) cleanup() {
	os.RemoveAll(c.tempDir)
}

// Write the specified contents on the mock of the specified cgroup files.
func (c *CgroupTestUtil) writeFileContents(fileContents map[string]string) {
	for file, contents := range fileContents {
		err := cgroups.WriteFile(c.CgroupPath, file, contents)
		if err != nil {
			c.t.Fatal(err)
		}
	}
}

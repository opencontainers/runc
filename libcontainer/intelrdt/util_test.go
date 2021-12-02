/*
 * Utility for testing Intel RDT operations.
 * Creates a mock of the Intel RDT "resource control" filesystem for the duration of the test.
 */
package intelrdt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

type intelRdtTestUtil struct {
	config *configs.Config

	// Path to the mock Intel RDT "resource control" filesystem directory
	IntelRdtPath string

	t *testing.T
}

// Creates a new test util
func NewIntelRdtTestUtil(t *testing.T) *intelRdtTestUtil {
	config := &configs.Config{
		IntelRdt: &configs.IntelRdt{},
	}

	// Assign fake intelRtdRoot value, returned by Root().
	intelRdtRoot = t.TempDir()
	// Make sure Root() won't even try to parse mountinfo.
	rootOnce.Do(func() {})

	testIntelRdtPath := filepath.Join(intelRdtRoot, "resctrl")

	// Ensure the full mock Intel RDT "resource control" filesystem path exists
	if err := os.MkdirAll(testIntelRdtPath, 0o755); err != nil {
		t.Fatal(err)
	}
	return &intelRdtTestUtil{config: config, IntelRdtPath: testIntelRdtPath, t: t}
}

// Write the specified contents on the mock of the specified Intel RDT "resource control" files
func (c *intelRdtTestUtil) writeFileContents(fileContents map[string]string) {
	for file, contents := range fileContents {
		err := writeFile(c.IntelRdtPath, file, contents)
		if err != nil {
			c.t.Fatal(err)
		}
	}
}

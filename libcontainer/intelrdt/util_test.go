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

	// Path to the mock pre-existing CLOS (or resctrl root if no CLOS is specified)
	IntelRdtPath string

	t *testing.T
}

// Creates a new test util. If mockClosName is non-empty, a mock CLOS with that name will be created.
func NewIntelRdtTestUtil(t *testing.T, mockClosName string) *intelRdtTestUtil {
	config := &configs.Config{
		IntelRdt: &configs.IntelRdt{},
	}

	// Assign fake intelRtdRoot value, returned by Root().
	intelRdtRoot = t.TempDir()
	// Make sure Root() won't even try to parse mountinfo.
	rootOnce.Do(func() {})

	testIntelRdtPath := filepath.Join(intelRdtRoot, mockClosName)

	// Ensure the mocked CLOS exists
	if err := os.MkdirAll(filepath.Join(testIntelRdtPath, "mon_groups"), 0o755); err != nil {
		t.Fatal(err)
	}

	return &intelRdtTestUtil{config: config, IntelRdtPath: testIntelRdtPath, t: t}
}

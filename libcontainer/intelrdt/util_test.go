/*
 * Utility for testing Intel RDT operations.
 * Creates a mock of the Intel RDT "resource control" filesystem for the duration of the test.
 */
package intelrdt

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeRoot creates a new fake root for tests and returns its path.
// Once this is called, Root() returns the same path.
func fakeRoot(t *testing.T) string {
	// Assign fake intelRtdRoot value, returned by Root().
	intelRdtRoot := filepath.Join(t.TempDir(), "resctrl")
	if err := os.MkdirAll(intelRdtRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	origRoot := root
	t.Cleanup(func() {
		root = origRoot
	})

	root = func() (string, error) {
		return intelRdtRoot, nil
	}

	return intelRdtRoot
}

package exeseal

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// AnnotationKey is the OCI annotation that selects how runc protects
// the host runc binary against tampering. See ValidateMode for the
// recognized values.
const AnnotationKey = "org.opencontainers.runc.clone-self-exe"

const (
	ModeUnset               = ""
	ModeIndependentDataCopy = "independent-data-copy"
	ModeROSharedPage        = "ro-shared-page"
)

// ValidateMode reports whether value is a recognized annotation value.
//
// Recognized values:
//   - "":                      annotation absent; use the default fallback chain.
//   - "independent-data-copy": use the clone-binary path (memfd, with
//     an internal fallback to a classic unlinked tmpfile on older
//     kernels). Sealed overlayfs is not attempted.
//   - "ro-shared-page":        use sealed overlayfs only; fail
//     container creation if it is not available.
func ValidateMode(value string) error {
	switch value {
	case ModeUnset, ModeIndependentDataCopy, ModeROSharedPage:
		return nil
	default:
		return fmt.Errorf("invalid %s value %q (want %q or %q)",
			AnnotationKey, value,
			ModeIndependentDataCopy, ModeROSharedPage)
	}
}

// strategy produces a sealed /proc/self/exe handle by one specific
// mechanism. Returns the file on success, or an error on failure.
type strategy func(tmpDir string) (*os.File, error)

func overlayfsStrategy(tmpDir string) (*os.File, error) {
	f, err := sealedOverlayfs("/proc/self/exe", tmpDir)
	if err != nil {
		return nil, err
	}
	logrus.Debug("runc exeseal: using overlayfs for sealed /proc/self/exe") // used for tests
	return f, nil
}

func cloneBinaryStrategy(tmpDir string) (*os.File, error) {
	selfExe, err := os.Open("/proc/self/exe")
	if err != nil {
		return nil, fmt.Errorf("opening current binary: %w", err)
	}
	defer selfExe.Close()

	stat, err := selfExe.Stat()
	if err != nil {
		return nil, fmt.Errorf("checking /proc/self/exe size: %w", err)
	}
	logrus.Debug("runc exeseal: using clone-binary path") // used for tests
	return CloneBinary(selfExe, stat.Size(), "/proc/self/exe", tmpDir)
}

// strategiesFor returns the ordered list of strategies to try for a
// given annotation value. The first successful strategy wins; if all
// fail, the last error is returned to the caller.
func strategiesFor(mode string) []strategy {
	switch mode {
	case ModeROSharedPage:
		return []strategy{overlayfsStrategy}
	case ModeIndependentDataCopy:
		return []strategy{cloneBinaryStrategy}
	case ModeUnset:
		// Historical default: overlayfs first, clone-binary fallback.
		// The order may be reversed in a future release.
		return []strategy{overlayfsStrategy, cloneBinaryStrategy}
	default:
		return nil
	}
}

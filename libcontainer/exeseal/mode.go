package exeseal

import "fmt"

// AnnotationKey is the OCI annotation that selects how runc protects
// the host runc binary against tampering by the container. See
// ParseMode for the recognized values and behavior.
const AnnotationKey = "org.opencontainers.runc.clone-self-exe"

// Mode the selected mechanism used to protect the host runc binary.
// See ParseMode for the recognized annotation values.
type Mode int

const (
	// ModeUnset means the annotation was not present in the config.
	ModeUnset Mode = iota
	ModeIndependentDataCopy
	ModeROSharedPage
)

// String returns the canonical annotation value for a Mode, or
// "<unset>" for ModeUnset (which has no annotation form).
func (m Mode) String() string {
	switch m {
	case ModeUnset:
		return "<unset>"
	case ModeIndependentDataCopy:
		return "independent-data-copy"
	case ModeROSharedPage:
		return "ro-shared-page"
	default:
		return fmt.Sprintf("unknown(%d)", int(m))
	}
}

// ParseMode converts an annotation value string into a Mode.
//
// Recognized values:
//   - "independent-data-copy": use the clone-binary path (memfd, with
//     an internal fallback to a classic unlinked tmpfile on older
//     kernels). Sealed overlayfs is not attempted.
//   - "ro-shared-page":        use sealed overlayfs only; fail
//     container creation if it is not available.
//
// If the annotation is absent, use ModeUnset.
//
// Explicit values do not fall back to the other mechanism on failure; this is intentional.
// if a caller has expressed a preference, getting the other mechanism silently defeats the
// purpose of the annotation.
//
// Unrecognized values, including empty string, return an error.
func ParseMode(value string) (Mode, error) {
	switch value {
	case "independent-data-copy":
		return ModeIndependentDataCopy, nil
	case "ro-shared-page":
		return ModeROSharedPage, nil
	default:
		return ModeUnset, fmt.Errorf(
			"invalid %s value %q (want %q or %q)",
			AnnotationKey, value,
			"independent-data-copy", "ro-shared-page",
		)
	}
}

// Package features provides the annotations for [github.com/opencontainers/runtime-spec/specs-go/features].
package features

const (
	// AnnotationRuncVersion represents the version of runc, e.g., "1.2.3", "1.2.3+dev", "1.2.3-rc.4.", "1.2.3-rc.4+dev".
	// Third party implementations such as crun and runsc MAY use this annotation to report the most compatible runc version,
	// however, parsing this annotation value is discouraged.
	AnnotationRuncVersion = "org.opencontainers.runc.version"

	// AnnotationRuncCommit corresponds to the output of `git describe --dirty --long --always` in the runc repo.
	// Third party implementations such as crun and runsc SHOULD NOT use this annotation, as their repo is different from the runc repo.
	// Parsing this annotation value is discouraged.
	AnnotationRuncCommit = "org.opencontainers.runc.commit"

	// AnnotationRuncCheckpointEnabled is set to "true" if CRIU-based checkpointing is supported.
	// Unrelated to whether the host supports CRIU or not.
	// Always set to "true" in the current version of runc.
	// This is defined as an annotation because checkpointing is a runc-specific feature that is not defined in the OCI Runtime Spec.
	// Third party implementations such as crun and runsc MAY use this annotation.
	AnnotationRuncCheckpointEnabled = "org.opencontainers.runc.checkpoint.enabled"

	// AnnotationLibseccompVersion is the version of libseccomp, e.g., "2.5.1".
	// Note that the runtime MAY support seccomp even when this annotation is not present.
	AnnotationLibseccompVersion = "io.github.seccomp.libseccomp.version"
)

// Package features provides the JSON structure that is printed by `runc features` (since runc v1.1.0).
// The types in this package are experimental and subject to change.
package features

// Features represents the supported features of the runtime.
type Features struct {
	// OCIVersionMin is the minimum OCI Runtime Spec version recognized by the runtime, e.g., "1.0.0".
	OCIVersionMin string `json:"ociVersionMin,omitempty"`

	// OCIVersionMax is the maximum OCI Runtime Spec version recognized by the runtime, e.g., "1.0.2-dev".
	OCIVersionMax string `json:"ociVersionMax,omitempty"`

	// Hooks is the list of the recognized hook names, e.g., "createRuntime".
	// Nil value means "unknown", not "no support for any hook".
	Hooks []string `json:"hooks,omitempty"`

	// MountOptions is the list of the recognized mount options, e.g., "ro".
	// Nil value means "unknown", not "no support for any mount option".
	// This list does not contain filesystem-specific options passed to mount(2) syscall as (const void *).
	MountOptions []string `json:"mountOptions,omitempty"`

	// Linux is specific to Linux.
	Linux *Linux `json:"linux,omitempty"`

	// Annotations contains implementation-specific annotation strings,
	// such as the implementation version, and third-party extensions.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// Linux is specific to Linux.
type Linux struct {
	// Namespaces is the list of the recognized namespaces, e.g., "mount".
	// Nil value means "unknown", not "no support for any namespace".
	Namespaces []string `json:"namespaces,omitempty"`

	// Capabilities is the list of the recognized capabilities , e.g., "CAP_SYS_ADMIN".
	// Nil value means "unknown", not "no support for any capability".
	Capabilities []string `json:"capabilities,omitempty"`

	Cgroup   *Cgroup   `json:"cgroup,omitempty"`
	Seccomp  *Seccomp  `json:"seccomp,omitempty"`
	Apparmor *Apparmor `json:"apparmor,omitempty"`
	Selinux  *Selinux  `json:"selinux,omitempty"`
}

// Seccomp represents the "seccomp" field.
type Seccomp struct {
	// Enabled is true if seccomp support is compiled in.
	// Nil value means "unknown", not "false".
	Enabled *bool `json:"enabled,omitempty"`

	// Actions is the list of the recognized actions, e.g., "SCMP_ACT_NOTIFY".
	// Nil value means "unknown", not "no support for any action".
	Actions []string `json:"actions,omitempty"`

	// Operators is the list of the recognized actions, e.g., "SCMP_CMP_NE".
	// Nil value means "unknown", not "no support for any operator".
	Operators []string `json:"operators,omitempty"`

	// Operators is the list of the recognized archs, e.g., "SCMP_ARCH_X86_64".
	// Nil value means "unknown", not "no support for any arch".
	Archs []string `json:"archs,omitempty"`
}

// Apparmor represents the "apparmor" field.
type Apparmor struct {
	// Enabled is true if AppArmor support is compiled in.
	// Unrelated to whether the host supports AppArmor or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	Enabled *bool `json:"enabled,omitempty"`
}

// Selinux represents the "selinux" field.
type Selinux struct {
	// Enabled is true if SELinux support is compiled in.
	// Unrelated to whether the host supports SELinux or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	Enabled *bool `json:"enabled,omitempty"`
}

// Cgroup represents the "cgroup" field.
type Cgroup struct {
	// V1 represents whether Cgroup v1 support is compiled in.
	// Unrelated to whether the host uses cgroup v1 or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	V1 *bool `json:"v1,omitempty"`

	// V2 represents whether Cgroup v2 support is compiled in.
	// Unrelated to whether the host uses cgroup v2 or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	V2 *bool `json:"v2,omitempty"`

	// Systemd represents whether systemd-cgroup support is compiled in.
	// Unrelated to whether the host uses systemd or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	Systemd *bool `json:"systemd,omitempty"`

	// SystemdUser represents whether user-scoped systemd-cgroup support is compiled in.
	// Unrelated to whether the host uses systemd or not.
	// Nil value means "unknown", not "false".
	// Always true in the current version of runc.
	SystemdUser *bool `json:"systemdUser,omitempty"`
}

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

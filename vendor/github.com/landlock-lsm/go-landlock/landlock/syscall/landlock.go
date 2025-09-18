// Package syscall provides a low-level interface to the Linux Landlock sandboxing feature.
//
// The package contains constants and syscall wrappers. The syscall
// wrappers whose names start with AllThreads will execute the syscall
// on all OS threads belonging to the current process, as long as
// these threads have been started implicitly by the Go runtime or
// using `pthread_create`.
//
// This package package is a stopgap solution while there is no
// Landlock support in x/sys/unix. The syscall package is considered
// highly unstable and may change or disappear without warning.
//
// The full documentation can be found at
// https://www.kernel.org/doc/html/latest/userspace-api/landlock.html.
package syscall

// Landlock file system access rights.
//
// Please see the full documentation at
// https://www.kernel.org/doc/html/latest/userspace-api/landlock.html#filesystem-flags.
const (
	AccessFSExecute = 1 << iota
	AccessFSWriteFile
	AccessFSReadFile
	AccessFSReadDir
	AccessFSRemoveDir
	AccessFSRemoveFile
	AccessFSMakeChar
	AccessFSMakeDir
	AccessFSMakeReg
	AccessFSMakeSock
	AccessFSMakeFifo
	AccessFSMakeBlock
	AccessFSMakeSym
	AccessFSRefer
	AccessFSTruncate
	AccessFSIoctlDev
)

// Landlock network access rights.
//
// Please see the full documentation at
// https://www.kernel.org/doc/html/latest/userspace-api/landlock.html#network-flags.
const (
	AccessNetBindTCP = 1 << iota
	AccessNetConnectTCP
)

// RulesetAttr is the Landlock ruleset definition.
//
// Argument of LandlockCreateRuleset(). This structure can grow in future versions of Landlock.
//
// C version is in usr/include/linux/landlock.h
type RulesetAttr struct {
	HandledAccessFS  uint64
	HandledAccessNet uint64
}

// The size of the RulesetAttr struct in bytes.
const rulesetAttrSize = 16

// PathBeneathAttr references a file hierarchy and defines the desired
// extent to which it should be usable when the rule is enforced.
type PathBeneathAttr struct {
	// AllowedAccess is a bitmask of allowed actions for this file
	// hierarchy (cf. "Filesystem flags"). The enabled bits must
	// be a subset of the bits defined in the ruleset.
	AllowedAccess uint64

	// ParentFd is a file descriptor, opened with `O_PATH`, which identifies
	// the parent directory of a file hierarchy, or just a file.
	ParentFd int
}

// NetPortAttr specifies which ports can be used for what.
type NetPortAttr struct {
	AllowedAccess uint64
	Port          uint64
}

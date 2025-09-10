// Package landlock restricts a Go program's ability to use files and networking.
//
// # Restricting file access
//
// The following invocation will restrict all goroutines so that they
// can only read from /usr, /bin and /tmp, and only write to /tmp:
//
//	err := landlock.V5.BestEffort().RestrictPaths(
//	    landlock.RODirs("/usr", "/bin"),
//	    landlock.RWDirs("/tmp"),
//	)
//
// This will restrict file access using Landlock V5, if available. If
// unavailable, it will attempt using earlier Landlock versions than
// the one requested. If no Landlock version is available, it will
// still succeed, without restricting file accesses.
//
// # Restricting networking
//
// The following invocation will restrict all goroutines so that they
// can only bind to TCP port 8080 and only connect to TCP port 53:
//
//	err := landlock.V5.BestEffort().RestrictNet(
//	    landlock.BindTCP(8080),
//	    landlock.ConnectTCP(53),
//	)
//
// This functionality is available since Landlock V5.
//
// # Restricting file access and networking at once
//
// The following invocation restricts both file and network access at
// once.  The effect is the same as calling [Config.RestrictPaths] and
// [Config.RestrictNet] one after another, but it happens in one step.
//
//	err := landlock.V5.BestEffort().Restrict(
//	    landlock.RODirs("/usr", "/bin"),
//	    landlock.RWDirs("/tmp"),
//	    landlock.BindTCP(8080),
//	    landlock.ConnectTCP(53),
//	)
//
// # More possible invocations
//
// landlock.V5.RestrictPaths(...) (without the call to
// [Config.BestEffort]) enforces the given rules using the
// capabilities of Landlock V5, but returns an error if that
// functionality is not available on the system that the program is
// running on.
//
// # Landlock ABI versioning
//
// The Landlock ABI is versioned, so that callers can probe for the
// availability of different Landlock features.
//
// When using the Go Landlock package, callers need to identify at
// which ABI level they want to use Landlock and call one of the
// restriction methods (e.g. [Config.RestrictPaths]) on the
// corresponding ABI constant.
//
// When new Landlock versions become available in landlock, users will
// manually need to upgrade their usages to higher Landlock versions,
// as there is a risk that new Landlock versions will break operations
// that their programs rely on.
//
// # Graceful degradation on older kernels
//
// Programs that get run on different kernel versions will want to use
// the [Config.BestEffort] method to gracefully degrade to using the
// best available Landlock version on the current kernel.
//
// In this case, the Go Landlock library will enforce as much as
// possible, but it will ensure that all the requested access rights
// are permitted after Landlock enforcement.
//
// # Current limitations
//
// Landlock can not currently restrict all file system operations.
// The operations that can and can not be restricted yet are listed in
// the [Kernel Documentation about Access Rights].
//
// Enabling Landlock implicitly turns off the following file system
// features:
//
//   - File reparenting: renaming or linking a file to a different parent directory is denied,
//     unless it is explicitly enabled on both directories with the "Refer" access modifier,
//     and the new target directory does not grant the file additional rights through its
//     Landlock access rules.
//   - Filesystem topology modification: arbitrary mounts are always denied.
//
// These are Landlock limitations that will be resolved in future
// versions. See the [Kernel Documentation about Current Limitations]
// for more details.
//
// # Multithreading Limitations
//
// This warning only applies to programs using cgo and linking C
// libraries that start OS threads through means other than
// pthread_create() before landlock is called:
//
// When using cgo, the landlock package relies on libpsx in order to
// apply the rules across all OS threads, (rather than just the ones
// managed by the Go runtime). psx achieves this by wrapping the
// C-level phtread_create() API which is very commonly used on Unix to
// start threads. However, C libraries calling clone(2) through other
// means before landlock is called might still create threads that
// won't have Landlock protections.
//
// [Kernel Documentation about Access Rights]: https://www.kernel.org/doc/html/latest/userspace-api/landlock.html#access-rights
// [Kernel Documentation about Current Limitations]: https://www.kernel.org/doc/html/latest/userspace-api/landlock.html#current-limitations
package landlock

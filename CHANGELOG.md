# Changelog/
This file documents all notable changes made to this project since runc 1.0.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Deprecated

 * `runc` option `--criu` is now ignored (with a warning), and the option will
   be removed entirely in a future release. Users who need a non-standard
   `criu` binary should rely on the standard way of looking up binaries in
   `$PATH`. (#3316)

### Changed

 * When Intel RDT feature is not available, its initialization is skipped,
   resulting in slightly faster `runc exec` and `runc run`. (#3306)

### Fixed

 * In case the runc binary resides on tmpfs, `runc init` no longer re-execs
   itself twice. (#3342)

## [1.1.0] - 2022-01-14

> A plan depends as much upon execution as it does upon concept.

### Changed
 * libcontainer will now refuse to build without the nsenter package being
   correctly compiled (specifically this requires CGO to be enabled). This
   should avoid folks accidentally creating broken runc binaries (and
   incorrectly importing our internal libraries into their projects). (#3331)

## [1.1.0-rc.1] - 2021-12-14

> He who controls the spice controls the universe.

### Deprecated
 * runc run/start now warns if a new container cgroup is non-empty or frozen;
   this warning will become an error in runc 1.2. (#3132, #3223)
 * runc can only be built with Go 1.16 or later from this release onwards.
   (#3100, #3245, #3325)

### Removed
 * `cgroup.GetHugePageSizes` has been removed entirely, and been replaced with
   `cgroup.HugePageSizes` which is more efficient. (#3234)
 * `intelrdt.GetIntelRdtPath` has been removed. Users who were using this
   function to get the intelrdt root should use the new `intelrdt.Root`
   instead. (#2920, #3239)

### Added
 * Add support for RDMA cgroup added in Linux 4.11. (#2883)
 * runc exec now produces exit code of 255 when the exec failed.
   This may help in distinguishing between runc exec failures
   (such as invalid options, non-running container or non-existent
   binary etc.) and failures of the command being executed. (#3073)
 * runc run: new `--keep` option to skip removal exited containers artefacts.
   This might be useful to check the state (e.g. of cgroup controllers) after
   the container has￼exited. (#2817, #2825)
 * seccomp: add support for `SCMP_ACT_KILL_PROCESS` and `SCMP_ACT_KILL_THREAD`
   (the latter is just an alias for `SCMP_ACT_KILL`). (#3204)
 * seccomp: add support for `SCMP_ACT_NOTIFY` (seccomp actions). This allows
   users to create sophisticated seccomp filters where syscalls can be
   efficiently emulated by privileged processes on the host. (#2682)
 * checkpoint/restore: add an option (`--lsm-mount-context`) to set
   a different LSM mount context on restore. (#3068)
 * runc releases are now cross-compiled for several architectures. Static
   builds for said architectures will be available for all future releases.
   (#3197)
 * intelrdt: support ClosID parameter. (#2920)
 * runc exec --cgroup: an option to specify a (non-top) in-container cgroup
   to use for the process being executed. (#3040, #3059)
 * cgroup v1 controllers now support hybrid hierarchy (i.e. when on a cgroup v1
   machine a cgroup2 filesystem is mounted to /sys/fs/cgroup/unified, runc
   run/exec now adds the container to the appropriate cgroup under it). (#2087,
   #3059)
 * sysctl: allow slashes in sysctl names, to better match `sysctl(8)`'s
   behaviour. (#3254, #3257)
 * mounts: add support for bind-mounts which are inaccessible after switching
   the user namespace. Note that this does not permit the container any
   additional access to the host filesystem, it simply allows containers to
   have bind-mounts configured for paths the user can access but have
   restrictive access control settings for other users. (#2576)
 * Add support for recursive mount attributes using `mount_setattr(2)`. These
   have the same names as the proposed `mount(8)` options -- just prepend `r`
   to the option name (such as `rro`). (#3272)
 * Add `runc features` subcommand to allow runc users to detect what features
   runc has been built with. This includes critical information such as
   supported mount flags, hook names, and so on. Note that the output of this
   command is subject to change and will not be considered stable until runc
   1.2 at the earliest. The runtime-spec specification for this feature is
   being developed in [opencontainers/runtime-spec#1130]. (#3296)

[opencontainers/runtime-spec#1130]: https://github.com/opencontainers/runtime-spec/pull/1130

### Changed
 * system: improve performance of `/proc/$pid/stat` parsing. (#2696)
 * cgroup2: when `/sys/fs/cgroup` is configured as a read-write mount, change
   the ownership of certain cgroup control files (as per
   `/sys/kernel/cgroup/delegate`) to allow for proper deferral to the container
   process. (#3057)
 * docs: series of improvements to man pages to make them easier to read and
   use. (#3032)

#### libcontainer API
 * internal api: remove internal error types and handling system, switch to Go
   wrapped errors. (#3033)
 * New configs.Cgroup structure fields (#3177):
   * Systemd (whether to use systemd cgroup manager); and
   * Rootless (whether to use rootless cgroups).
 * New cgroups/manager package aiming to simplify cgroup manager instantiation.
   (#3177)
 * All cgroup managers' instantiation methods now initialize cgroup paths and
   can return errors. This allows to use any cgroup manager method (e.g.
   Exists, Destroy, Set, GetStats) right after instantiation, which was not
   possible before (as paths were initialized in Apply only). (#3178)

### Fixed
 * nsenter: do not try to close already-closed fds during container setup and
   bail on close(2) failures. (#3058)
 * runc checkpoint/restore: fixed for containers with an external bind mount
   which destination is a symlink. (#3047).
 * cgroup: improve openat2 handling for cgroup directory handle hardening.
   (#3030)
 * `runc delete -f` now succeeds (rather than timing out) on a paused
   container. (#3134)
 * runc run/start/exec now refuses a frozen cgroup (paused container in case of
   exec). Users can disable this using `--ignore-paused`. (#3132, #3223)
 * config: do not permit null bytes in mount fields. (#3287)


## [1.0.3] - 2021-12-06

> If you were waiting for the opportune moment, that was it.

### Security
 * A potential vulnerability was discovered in runc (related to an internal
   usage of netlink), however upon further investigation we discovered that
   while this bug was exploitable on the master branch of runc, no released
   version of runc could be exploited using this bug. The exploit required being
   able to create a netlink attribute with a length that would overflow a uint16
   but this was not possible in any released version of runc. For more
   information, see [GHSA-v95c-p5hm-xq8f][] and CVE-2021-43784.

### Fixed
 * Fixed inability to start a container with read-write bind mount of a
   read-only fuse host mount. (#3283, #3292)
 * Fixed inability to start when read-only /dev in set in spec (#3276, #3277)
 * Fixed not removing sub-cgroups upon container delete, when rootless cgroup v2
   is used with older systemd. (#3226, #3297)
 * Fixed returning error from GetStats when hugetlb is unsupported (which causes
   excessive logging for Kubernetes). (#3233, #3295)
 * Improved an error message when dbus-user-session is not installed and
   rootless + cgroup2 + systemd are used (#3212)

[GHSA-v95c-p5hm-xq8f]: https://github.com/opencontainers/runc/security/advisories/GHSA-v95c-p5hm-xq8f


## [1.0.2] - 2021-07-16

> Given the right lever, you can move a planet.

### Changed
 * Made release builds reproducible from now on. (#3099, #3142)

### Fixed
 * Fixed a failure to set CPU quota period in some cases on cgroup v1. (#3090
   #3115)
 * Fixed the inability to start a container with the "adding seccomp filter
   rule for syscall ..." error, caused by redundant seccomp rules (i.e. those
   that has action equal to the default one). Such redundant rules are now
   skipped. (#3109, #3129)
 * Fixed a rare debug log race in runc init, which can result in occasional
   harmful "failed to decode ..." errors from runc run or exec. (#3120, #3130)
 * Fixed the check in cgroup v1 systemd manager if a container needs to be
   frozen before Set, and add a setting to skip such freeze unconditionally.
   The previous fix for that issue, done in  runc 1.0.1, was not working.
   (#3166, #3167)


## [1.0.1] - 2021-07-16

> If in doubt, Meriadoc, always follow your nose.

### Fixed
 * Fixed occasional runc exec/run failure ("interrupted system call") on an
   Azure volume. (#3045, #3074)
 * Fixed "unable to find groups ... token too long" error with /etc/group
   containing lines longer than 64K characters. (#3062, #3079)
 * cgroup/systemd/v1: fix leaving cgroup frozen after Set if a parent cgroup is
   frozen.  This is a regression in 1.0.0, not affecting runc itself but some
   of libcontainer users (e.g Kubernetes). (#3081, #3085)
 * cgroupv2: bpf: Ignore inaccessible existing programs in case of
   permission error when handling replacement of existing bpf cgroup
   programs. This fixes a regression in 1.0.0, where some SELinux
   policies would block runc from being able to run entirely. (#3055, #3087)
 * cgroup/systemd/v2: don't freeze cgroup on Set. (#3067, #3092)
 * cgroup/systemd/v1: avoid unnecessary freeze on Set. (#3082, #3093)


## [1.0.0] - 2021-06-22

> A wizard is never late, nor is he early, he arrives precisely when he means
> to.

As runc follows Semantic Versioning, we will endeavour to not make any
breaking changes without bumping the major version number of runc.
However, it should be noted that Go API usage of runc's internal
implementation (libcontainer) is *not* covered by this policy.

### Removed
 * Removed libcontainer/configs.Device* identifiers (deprecated since rc94,
   use libcontainer/devices). (#2999)
 * Removed libcontainer/system.RunningInUserNS function (deprecated since
   rc94, use libcontainer/userns). (#2999)

### Deprecated
 * The usage of relative paths for mountpoints will now produce a warning
   (such configurations are outside of the spec, and in future runc will
   produce an error when given such configurations). (#2917, #3004)

### Fixed
 * cgroupv2: devices: rework the filter generation to produce consistent
   results with cgroupv1, and always clobber any existing eBPF
   program(s) to fix `runc update` and avoid leaking eBPF programs
   (resulting in errors when managing containers).  (#2951)
 * cgroupv2: correctly convert "number of IOs" statistics in a
   cgroupv1-compatible way. (#2965, #2967, #2968, #2964)
 * cgroupv2: support larger than 32-bit IO statistics on 32-bit architectures.
 * cgroupv2: wait for freeze to finish before returning from the freezing
   code, optimize the method for checking whether a cgroup is frozen. (#2955)
 * cgroups/systemd: fixed "retry on dbus disconnect" logic introduced in rc94
 * cgroups/systemd: fixed returning "unit already exists" error from a systemd
   cgroup manager (regression in rc94) (#2997, #2996)

### Added
 * cgroupv2: support SkipDevices with systemd driver. (#2958, #3019)
 * cgroup1: blkio: support BFQ weights. (#3010)
 * cgroupv2: set per-device io weights if BFQ IO scheduler is available.
   (#3022)

### Changed
 * cgroup/systemd: return, not ignore, stop unit error from Destroy (#2946)
 * Fix all golangci-lint failures. (#2781, #2962)
 * Make `runc --version` output sane even when built with `go get` or
   otherwise outside of our build scripts. (#2962)
 * cgroups: set SkipDevices during runc update (so we don't modify
   cgroups at all during `runc update`). (#2994)

<!-- minor releases -->
[Unreleased]: https://github.com/opencontainers/runc/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/opencontainers/runc/compare/v1.1.0-rc.1...v1.1.0
[1.0.0]: https://github.com/opencontainers/runc/releases/tag/v1.0.0

<!-- 1.0.z patch releases -->
[Unreleased 1.0.z]: https://github.com/opencontainers/runc/compare/v1.0.3...release-1.0
[1.0.3]: https://github.com/opencontainers/runc/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/opencontainers/runc/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/opencontainers/runc/compare/v1.0.0...v1.0.1

<!-- 1.1.z patch releases -->
[Unreleased 1.1.z]: https://github.com/opencontainers/runc/compare/v1.1.0...release-1.1
[1.1.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.0.0...v1.1.0-rc.1

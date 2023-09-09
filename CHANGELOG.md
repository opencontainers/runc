# Changelog
This file documents all notable changes made to this project since runc 1.0.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Deprecated

 * `runc` option `--criu` is now ignored (with a warning), and the option will
   be removed entirely in a future release. Users who need a non-standard
   `criu` binary should rely on the standard way of looking up binaries in
   `$PATH`. (#3316)
 * `runc kill` option `-a` is now deprecated. Previously, it had to be specified
   to kill a container (with SIGKILL) which does not have its own private PID
   namespace (so that runc would send SIGKILL to all processes). Now, this is
   done automatically. (#3864, #3825)

### Changed

 * When Intel RDT feature is not available, its initialization is skipped,
   resulting in slightly faster `runc exec` and `runc run`. (#3306)
 * Enforce absolute paths for mounts. (#3020, #3717)
 * libcontainer users that create and kill containers from a daemon process
   (so that the container init is a child of that process) must now implement
   a proper child reaper in case a container does not have its own private PID
   namespace, as documented in `container.Signal`. (#3825)
 * Sum `anon` and `file` from `memory.stat` for cgroupv2 root usage,
   as the root does not have `memory.current` for cgroupv2.
   This aligns cgroupv2 root usage more closely with cgroupv1 reporting.
   Additionally, report root swap usage as sum of swap and memory usage,
   aligned with v1 and existing non-root v2 reporting. (#3933)
 * Add `swapOnlyUsage` in `MemoryStats`. This field reports swap-only usage.
   For cgroupv1, `Usage` and `Failcnt` are set by subtracting memory usage
   from memory+swap usage. For cgroupv2, `Usage`, `Limit`, and `MaxUsage`
   are set. (#4010)

### Fixed

 * In case the runc binary resides on tmpfs, `runc init` no longer re-execs
   itself twice. (#3342)
 * Our seccomp `-ENOSYS` stub now correctly handles multiplexed syscalls on
   s390 and s390x. This solves the issue where syscalls the host kernel did not
   support would return `-EPERM` despite the existence of the `-ENOSYS` stub
   code (this was due to how s390x does syscall multiplexing). (#3474)
 * Remove tun/tap from the default device rules. (#3468)
 * specconv: avoid mapping "acl" to MS_POSIXACL. (#3739)

## [1.1.8] - 2023-07-20

> 海纳百川 有容乃大

### Added

* Support riscv64. (#3905)

### Fixed

* init: do not print environment variable value. (#3879)
* libct: fix a race with systemd removal. (#3877)
* tests/int: increase num retries for oom tests. (#3891)
* man/runc: fixes. (#3892)
* Fix tmpfs mode opts when dir already exists. (#3916)
* docs/systemd: fix a broken link. (#3917)
* ci/cirrus: enable some rootless tests on cs9. (#3918)
* runc delete: call systemd's reset-failed. (#3932)
* libct/cg/sd/v1: do not update non-frozen cgroup after frozen failed. (#3921)

### Changed

* CI: bump Fedora, Vagrant, bats. (#3878)
* `.codespellrc`: update for 2.2.5. (#3909)

## [1.1.7] - 2023-04-26

> Ночевала тучка золотая на груди утеса-великана.

### Fixed

* When used with systemd v240+, systemd cgroup drivers no longer skip
  `DeviceAllow` rules if the device does not exist (a regression introduced
  in runc 1.1.3). This fix also reverts the workaround added in runc 1.1.5,
  removing an extra warning emitted by runc run/start. (#3845, #3708, #3671)

### Added

* The source code now has a new file, `runc.keyring`, which contains the keys
  used to sign runc releases. (#3838)

## [1.1.6] - 2023-04-11

> In this world nothing is certain but death and taxes.

### Compatibility

* This release can no longer be built from sources using Go 1.16. Using a
  latest maintained Go 1.20.x or Go 1.19.x release is recommended.
  Go 1.17 can still be used.

### Fixed

* systemd cgroup v1 and v2 drivers were deliberately ignoring `UnitExist` error
  from systemd while trying to create a systemd unit, which in some scenarios
  may result in a container not being added to the proper systemd unit and
  cgroup. (#3780, #3806)
* systemd cgroup v2 driver was incorrectly translating cpuset range from spec's
  `resources.cpu.cpus` to systemd unit property (`AllowedCPUs`) in case of more
  than 8 CPUs, resulting in the wrong AllowedCPUs setting. (#3808)
* systemd cgroup v1 driver was prefixing container's cgroup path with the path
  of PID 1 cgroup, resulting in inability to place PID 1 in a non-root cgroup.
  (#3811)
* runc run/start may return "permission denied" error when starting a rootless
  container when the file to be executed does not have executable bit set for
  the user, not taking the `CAP_DAC_OVERRIDE` capability into account. This is
  a regression in runc 1.1.4, as well as in Go 1.20 and 1.20.1 (#3715, #3817)
* cgroup v1 drivers are now aware of `misc` controller. (#3823)
* Various CI fixes and improvements, mostly to ensure Go 1.19.x and Go 1.20.x
  compatibility.

## [1.1.5] - 2023-03-29

> 囚われた屈辱は
> 反撃の嚆矢だ

### Security

The following CVEs were fixed in this release:

* [CVE-2023-25809][] is a vulnerability involving rootless containers where
  (under specific configurations), the container would have write access to the
  `/sys/fs/cgroup/user.slice/...` cgroup hierarchy. No other hierarchies on the
  host were affected. This vulnerability was discovered by Akihiro Suda.

* [CVE-2023-27561][] was a regression in our protections against tricky `/proc`
  and `/sys` configurations (where the container mountpoint is a symlink)
  causing us to be tricked into incorrectly configuring the container, which
  effectively re-introduced [CVE-2019-19921][]. This regression was present
  from v1.0.0-rc95 to v1.1.4 and was discovered by @Beuc. (#3785)

* [CVE-2023-28642][] is a different attack vector using the same regression
  as in [CVE-2023-27561][]. This was reported by Lei Wang.

[CVE-2019-19921]: https://github.com/advisories/GHSA-fh74-hm69-rqjw
[CVE-2023-25809]: https://github.com/opencontainers/runc/security/advisories/GHSA-m8cg-xc2p-r3fc
[CVE-2023-27561]: https://github.com/advisories/GHSA-vpvm-3wq2-2wvm
[CVE-2023-28642]: https://github.com/opencontainers/runc/security/advisories/GHSA-g2j6-57v7-gm8c

### Fixed

* Fix the inability to use `/dev/null` when inside a container. (#3620)
* Fix changing the ownership of host's `/dev/null` caused by fd redirection
  (a regression in 1.1.1). (#3674, #3731)
* Fix rare runc exec/enter unshare error on older kernels, including
  CentOS < 7.7. (#3776)
* nsexec: Check for errors in `write_log()`. (#3721)
* Various CI fixes and updates. (#3618, #3630, #3640, #3729)

## [1.1.4] - 2022-08-24

> If you look for perfection, you'll never be content.

### Fixed

* Fix mounting via wrong proc fd.
  When the user and mount namespaces are used, and the bind mount is followed by
  the cgroup mount in the spec, the cgroup was mounted using the bind mount's
  mount fd. (#3511)
* Switch `kill()` in `libcontainer/nsenter` to `sane_kill()`. (#3536)
* Fix "permission denied" error from `runc run` on `noexec` fs. (#3541)
* Fix failed exec after `systemctl daemon-reload`.
  Due to a regression in v1.1.3, the `DeviceAllow=char-pts rwm` rule was no
  longer added and was causing an error `open /dev/pts/0: operation not permitted: unknown`
  when systemd was reloaded. (#3554)
* Various CI fixes. (#3538, #3558, #3562)

## [1.1.3] - 2022-06-09

> In the beginning there was nothing, which exploded.

### Fixed
 * Our seccomp `-ENOSYS` stub now correctly handles multiplexed syscalls on
   s390 and s390x. This solves the issue where syscalls the host kernel did not
   support would return `-EPERM` despite the existence of the `-ENOSYS` stub
   code (this was due to how s390x does syscall multiplexing). (#3478)
 * Retry on dbus disconnect logic in libcontainer/cgroups/systemd now works as
   intended; this fix does not affect runc binary itself but is important for
   libcontainer users such as Kubernetes. (#3476)
 * Inability to compile with recent clang due to an issue with duplicate
   constants in libseccomp-golang. (#3477)
 * When using systemd cgroup driver, skip adding device paths that don't exist,
   to stop systemd from emitting warnings about those paths. (#3504)
 * Socket activation was failing when more than 3 sockets were used. (#3494)
 * Various CI fixes. (#3472, #3479)

### Added
 * Allow to bind mount /proc/sys/kernel/ns_last_pid to inside container. (#3493)

### Changed
 * runc static binaries are now linked against libseccomp v2.5.4. (#3481)


## [1.1.2] - 2022-05-11

> I should think I'm going to be a perpetual student.

### Security
 * A bug was found in runc where runc exec --cap executed processes with
   non-empty inheritable Linux process capabilities, creating an atypical Linux
   environment. For more information, see [GHSA-f3fp-gc8g-vw66][] and
   CVE-2022-29162.

### Changed
 * `runc spec` no longer sets any inheritable capabilities in the created
   example OCI spec (`config.json`) file.

[GHSA-f3fp-gc8g-vw66]: https://github.com/opencontainers/runc/security/advisories/GHSA-f3fp-gc8g-vw66


## [1.1.1] - 2022-03-28

> Violence is the last refuge of the incompetent.

### Added
 * CI is now also run on centos-stream-9. (#3436)

### Fixed
 * `runc run/start` can now run a container with read-only `/dev` in OCI spec,
   rather than error out. (#3355)
 * `runc exec` now ensures that `--cgroup` argument is a sub-cgroup. (#3403)
 * libcontainer systemd v2 manager no longer errors out if one of the files
   listed in `/sys/kernel/cgroup/delegate` do not exist in container's cgroup.
   (#3387, #3404)
 * Loose OCI spec validation to avoid bogus "Intel RDT is not supported" error.
   (#3406)
 * libcontainer/cgroups no longer panics in cgroup v1 managers if `stat`
   of `/sys/fs/cgroup/unified` returns an error other than ENOENT. (#3435)


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
   the container has exited. (#2817, #2825)
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
 * Fixed inability to start when read-only /dev in set in spec. (#3276, #3277)
 * Fixed not removing sub-cgroups upon container delete, when rootless cgroup v2
   is used with older systemd. (#3226, #3297)
 * Fixed returning error from GetStats when hugetlb is unsupported (which causes
   excessive logging for Kubernetes). (#3233, #3295)
 * Improved an error message when dbus-user-session is not installed and
   rootless + cgroup2 + systemd are used. (#3212)

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
   cgroup manager (regression in rc94). (#2997, #2996)

### Added
 * cgroupv2: support SkipDevices with systemd driver. (#2958, #3019)
 * cgroup1: blkio: support BFQ weights. (#3010)
 * cgroupv2: set per-device io weights if BFQ IO scheduler is available.
   (#3022)

### Changed
 * cgroup/systemd: return, not ignore, stop unit error from Destroy. (#2946)
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
[Unreleased 1.1.z]: https://github.com/opencontainers/runc/compare/v1.1.8...release-1.1
[1.1.8]: https://github.com/opencontainers/runc/compare/v1.1.7...v1.1.8
[1.1.7]: https://github.com/opencontainers/runc/compare/v1.1.6...v1.1.7
[1.1.6]: https://github.com/opencontainers/runc/compare/v1.1.5...v1.1.6
[1.1.5]: https://github.com/opencontainers/runc/compare/v1.1.4...v1.1.5
[1.1.4]: https://github.com/opencontainers/runc/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/opencontainers/runc/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/opencontainers/runc/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/opencontainers/runc/compare/v1.1.0...v1.1.1
[1.1.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.0.0...v1.1.0-rc.1

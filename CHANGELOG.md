# Changelog
This file documents all notable changes made to this project since runc 1.0.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### libcontainer API
- The deprecated `libcontainer/userns` package has been removed; use
  `github.com/moby/sys/userns` instead.

### Breaking ###
- The handling of `pids.limit` has been updated to match the newer guidance
  from the OCI runtime specification. In particular, now a maximum limit value
  of `0` will be treated as an actual limit (due to limitations with systemd,
  it will be treated the same as a limit value of `1`). We only expect users
  that explicitly set `pids.limit` to `0` will see a behaviour change.
  (opencontainers/cgroups#48, #4949)

### Fixed ###
- cgroups: provide iocost statistics for cgroupv2. (opencontainers/cgroups#43)
- cgroups: retry DBus connection when it fails with EAGAIN.
  (opencontainers/cgroups#45)
- cgroups: improve `cpuacct.usage_all` resilience when parsing data from
  patched kernels (such as the Tencent kernels). (opencontainers/cgroups#46,
  opencontainers/cgroups#50)

### Changed ###
- CI: All PRs now require a corresponding `CHANGELOG.md` change be included,
  which should increase the quality and accuracy of our changelogs going
  forward. (#5047)

## [1.4.0-rc.1] - 2025-09-05

> おめェもボスになったんだろぉ？

This version of runc requires Go 1.24 to build.

### libcontainer API
- The deprecated `libcontainer/user` package has been removed; use
  `github.com/moby/sys/user` instead. (#3999, #4617)
- `libcontainer/apparmor` variables containing public functions have been
  switched to wrapper functions. (#4725)

### Breaking
- runc update no longer allows `--l3-cache-schema` or `--mem-bw-schema` if
  `linux.intelRdt` was not present in the container’s original `config.json`.

  Without `linux.intelRdt` no CLOS (resctrl group) is created at container
  creation, so it is not possible to apply the updated options with `runc
  update`.

  Previously, this scenario did not work as expected. The `runc update` would
  create a new CLOS but fail to apply the schema, move only the init process
  (omitting children) to the new group, and leave the CLOS orphaned after
  container exit. (#4827)
- The deprecated `--criu` flag has been removed entirely, instead the `criu`
  binary in `$PATH` will be used. (#4722)

### Added
 * runc now supports the `linux.netDevices` field to allow for devices to be
   moved into container network namespaces seamlessly. (#4538)
 * `runc update` now supports per-device weight and iops cgroup limits. (#4775)
 * intel rdt: allow explicit assignment to root CLOS. (#4854)

### Fixed
 * Container processes will no longer inherit the CPU affinity of runc by
   default. Instead, the default CPU affinity of container processes will be
   the largest set of CPUs permitted by the container's cpuset cgroup and any
   other system restrictions (such as isolated CPUs). (#4041, #4815, #4858)
 * Use `chown(uid, -1)` when configuring the console inode, to avoid issues
   with unmapped GIDs. (#4679)
 * Add logging for the cases where failed keyring operations are ignored during
   setup. (#4676)
 * Optimise `runc exec` by avoiding calling into SELinux's `Set.*Label` when
   `processLabel` is not set. (#4354)
 * Fix mips64 builds for remap-rootfs. (#4723)
 * Setting `linux.rootfsPropagation` to `shared` or `unbindable` now functions
   properly. (#1755, #1815, #4724)
 * runc delete and runc stop can now correctly handle cases where runc
   create was killed during setup. Previously it was possible for the
   container to be in such a state that neither runc stop nor runc
   delete would be unable to kill or delete the container. (#4534,
   #4645, #4757)
 * Close seccomp agent connection to prevent resource leaks. (#4796)
 * `runc update` will no longer clear intelRdt state information. (#4828)
 * runc will now error out earlier if intelRdt is not enabled. (#4829)
 * Improve filesystem operations within intelRdt manager. (#4840, #4831)
 * Resolve a certain race between `runc create` and `runc delete` that would
   previously result in spurious errors. (#4735)
 * CI: skip bpf tests on misbehaving udev systems. (#4825)

### Changes
 * Use Go's built-in `pidfd_send_signal(2)` support when available. (#4666)
 * Make `state.json` 25% smaller. (#4685)
 * Migrate to Go 1.22+ features. (#4687, #4703)
 * Provide private wrappers around common syscalls to make `-EINTR` handling
   less cumbersome for the rest of runc. (#4697)
 * Ignore the dmem controller in our cgroup tests, as systemd does not
   yet support it. (#4806)
 * `/proc/net/dev` is no longer included in the permitted procfs overmount
   list. Its inclusion was almost certainly an error, and because
   `/proc/net` is a symlink to `/proc/self/net`, overmounting this was
   almost certainly never useful (and will be blocked by future kernel
   versions). (#4817)
 * Simplify the prepareCriuRestoreMounts logic for checkpoint-restore.
   (#4765)
 * Bump minimum Go version to 1.24. (#4851)
 * CI: migrate virtualised Fedora tests from Vagrant + Cirrus to Lima + GHA. We
   still use Cirrus for the AlmaLinux tests, since they can be run without
   virtualisation. (#4664)
 * CI: install fewer dependencies (#4671), bump shellcheck and bats versions
   (#4670).
 * CI: remove `toolchain` from `go.mod` and add a CI check to make sure it's
   never added accidentally. (#4717, #4721)
 * CI: do not allow `exclude` or `replace` directives in `go.mod`, to make sure
   that `go install` doesn't get accidentally broken. (#4750)
 * CI: fix exclusion rules and allow us to run jobs manually. (#4760)
 * CI: Switch to GitHub-hosted ARM runners. Thanks again to @alexellis
   for supporting runc's ARM CI up until now. (#4844, #4856)
 * Various dependency updates. (#4659, #4658, #4662, #4663, #4689, #4694,
   #4702, #4701, #4707, #4710, #4746, #4756, #4751, #4758, #4764, #4768, #4779,
   #4783, #4785, #4801, #4808, #4803, #4839, #4846, #4847, #4845, #4850, #4861,
   #4860)

## [1.3.2] - 2025-10-02

> Ночь, улица, фонарь, аптека...

### Changed
 * The conversion from cgroup v1 CPU shares to cgroup v2 CPU weight is
   improved to better fit default v1 and v2 values. (#4772, #4785, #4897)
 * Dependency github.com/opencontainers/cgroups updated from v0.0.1 to
   v0.0.4. (#4897)

### Fixed
 * runc state: fix occasional "cgroup.freeze: no such device" error.
   (#4798, #4808, #4897)
 * Fixed integration test failure on ppc64, caused by 64K page size so the
   kernel was rounding memory limit to 64K. (#4841, #4895, #4893)

## [1.3.1] - 2025-09-05

> この瓦礫の山でよぉ

### Fixed
 * Container processes will no longer inherit the CPU affinity of runc by
   default. Instead, the default CPU affinity of container processes will be
   the largest set of CPUs permitted by the container's cpuset cgroup and any
   other system restrictions (such as isolated CPUs). (#4041, #4815, #4858)
 * Setting `linux.rootfsPropagation` to `shared` or `unbindable` now functions
   properly. (#1755, #1815, #4724, #4789)
 * Close seccomp agent connection to prevent resource leaks. (#4796, #4799)
 * `runc delete` and `runc stop` can now correctly handle cases where `runc
   create` was killed during setup. Previously it was possible for the
   container to be in such a state that neither `runc stop` nor `runc delete`
   would be unable to kill or delete the container. (#4534, #4645, #4757,
   #4793)
 * `runc update` will no longer clear intelRdt state information. (#4828,
   #4833)
 * CI: Fix exclusion rules and allow us to run jobs manually. (#4760, #4763)

### Changed
 * Improvements to the deprecation warnings as part of the
   `github.com/opencontainers/cgroups` split. (#4784, #4788)
 * Ignore the dmem controller in our cgroup tests, as systemd does not yet
   support it. (#4806, #4811)
 * `/proc/net/dev` is no longer included in the permitted procfs overmount
   list. Its inclusion was almost certainly an error, and because `/proc/net`
   is a symlink to `/proc/self/net`, overmounting this was almost certainly
   never useful (and will be blocked by future kernel versions). (#4817, #4820)
 * Simplify the `prepareCriuRestoreMounts` logic for checkpoint-restore.
   (#4765, #4871)
 * CI: Bump `golangci-lint` to v2.1. (#4747, #4754)
 * CI: Switch to GitHub-hosted ARM runners. Thanks again to @alexellis for
   supporting runc's ARM CI up until now. (#4844, #4856, #4866)

## [1.3.0] - 2025-04-30

> Mr. President, we must not allow a mine shaft gap!

### Fixed
 * Removed preemptive "full access to cgroups" warning when calling `runc
   pause` or `runc unpause` as an unprivileged user without
   `--systemd-cgroups`. Now the warning is only emitted if an actual permission
   error was encountered. (#4709)
 * Several fixes to our CI, mainly related to AlmaLinux and CRIU. (#4670,
   #4728, #4736)

### Changed
 * In runc 1.2, we changed our mount behaviour to correctly handle clearing
   flags. However, the error messages we returned did not provide as much
   information to users about what clearing flags were conflicting with locked
   mount flags. We now provide more diagnostic information if there is an error
   when in the fallback path to handle locked mount flags. (#4734)
 * Upgrade our CI to use golangci-lint v2.0. (#4692)
 * `runc version` information is now filled in using `//go:embed` rather than
   being set through `Makefile`. This allows `go install` or other non-`make`
   builds to contain the correct version information. Note that `make
   EXTRA_VERSION=...` still works. (#418)
 * Remove `exclude` directives from our `go.mod` for broken `cilium/ebpf`
   versions. `v0.17.3` resolved the issue we had, and `exclude` directives are
   incompatible with `go install`. (#4748)

## [1.3.0-rc.2] - 2025-04-10

> Eppur si muove.

### Fixed
 * Use the container's `/etc/passwd` to set the `HOME` env var. After a refactor
   for 1.3, we were setting it reading the host's `/etc/passwd` file instead.
   (#4693, #4688)
 * Override `HOME` env var if it's set to the empty string. This fixes a
   regression after the same refactor for 1.3 and aligns the behavior with older
   versions of runc. (#4711)
 * Add time namespace to container config after checkpoint/restore. CRIU since
   version 3.14 uses a time namespace for checkpoint/restore, however it was not
   joining the time namespace in runc. (#4705)

## [1.3.0-rc.1] - 2025-03-04

> No tengo miedo al invierno, con tu recuerdo lleno de sol.

### libcontainer API
 * `configs.CommandHook` struct has changed, Command is now a pointer.
   Also, `configs.NewCommandHook` now accepts a `*Command`. (#4325)
 * The `Process` struct has `User` string field replaced with numeric
   `UID` and `GID` fields, and `AdditionalGroups` changed its type from
   `[]string` to `[]int`. Essentially, resolution of user and group
   names to IDs is no longer performed by libcontainer, so if a libcontainer
   user previously relied on this feature, now they have to convert names to
   IDs before calling libcontainer; it is recommended to use Go package
   github.com/moby/sys/user for that. (#3999)
 * Move libcontainer/cgroups to a separate repository. (#4618)

### Fixed
 * `runc exec -p` no longer ignores specified `ioPriority` and `scheduler`
   settings. Similarly, libcontainer's `Container.Start` and `Container.Run`
   methods no longer ignore `Process.IOPriority` and `Process.Scheduler`
   settings. (#4585)
 * We no longer use `F_SEAL_FUTURE_WRITE` when sealing the runc binary, as it
   turns out this had some unfortunate bugs in older kernel versions and was
   never necessary in the first place. (#4641, #4640)
 * runc now uses a more flexible method of joining namespaces, which better
   matches the behaviour of `nsenter(8)`. This is mainly useful for users that
   create a container with a runc-managed user namespace but want the container
   to join some externally-managed namespace as well. (#4492)
 * `runc` now properly handles joining time namespaces (such as with `runc
   exec`). Previously we would attempt to set the time offsets when joining,
   which would fail. (#4635, #4636)
 * Handle `EINTR` retries correctly for socket-related direct
   `golang.org/x/sys/unix` system calls. (#4637)
 * Handle `close_range(2)` errors more gracefully. (#4596)
 * Fix a stall issue that would happen if setting `O_CLOEXEC` with
   `CloseExecFrom` failed (#4599).
 * Handle errors on older kernels when resetting ambient capabilities more
   gracefully. (#4597)

### Changed
 * runc now has an official release policy to help provide more consistency
   around our release schedules and better define our support policy for old
   release branches. See `RELEASES.md` for more details. (#4557)
 * Improved performance by switching to `strings.Cut` where appropriate.
   (#4470)
 * The minimum Go version of runc is now Go 1.23. (#4598)
 * Updated builds to libseccomp v2.5.6. (#4625)

### Added
 * runc has been updated to support OCI runtime-spec 1.2.1. (#4653)
 * CPU affinity support for `runc exec`. (#4327)
 * CRIU support can be disabled using the build tag `runc_nocriu`. (#4546)
 * Support to get the pidfd of the container via CLI flag `pidfd-socket`.
   (#4045)
 * Support `skip-in-flight` and `link-remap` options for CRIU. (#4627)
 * Support cgroup v1 mounted with `noprefix`. (#4513)

## [1.2.7] - 2025-09-05

> さんをつけろよデコ助野郎！

### Fixed
 * Removed preemptive "full access to cgroups" warning when calling `runc
   pause` or `runc unpause` as an unprivileged user without
   `--systemd-cgroups`. Now the warning is only emitted if an actual permission
   error was encountered. (#4709, #4720)
 * Add time namespace to container config after checkpoint/restore. CRIU since
   version 3.14 uses a time namespace for checkpoint/restore, however it was
   not joining the time namespace in runc. (#4696, #4714)
 * Container processes will no longer inherit the CPU affinity of runc by
   default. Instead, the default CPU affinity of container processes will be
   the largest set of CPUs permitted by the container's cpuset cgroup and any
   other system restrictions (such as isolated CPUs). (#4041, #4815, #4858)
 * Close seccomp agent connection to prevent resource leaks. (#4796, #4800)
 * Several fixes to our CI, mainly related to AlmaLinux and CRIU. (#4670,
   #4728, #4736, #4742)
 * Setting `linux.rootfsPropagation` to `shared` or `unbindable` now functions
   properly. (#1755, #1815, #4724, #4791)
 * `runc update` will no longer clear intelRdt state information. (#4828,
   #4834)

### Changed
 * In runc 1.2, we changed our mount behaviour to correctly handle clearing
   flags. However, the error messages we returned did not provide as much
   information to users about what clearing flags were conflicting with locked
   mount flags. We now provide more diagnostic information if there is an error
   when in the fallback path to handle locked mount flags. (#4734, #4740)
 * Ignore the dmem controller in our cgroup tests, as systemd does not yet
   support it. (#4806, #4811)
 * `/proc/net/dev` is no longer included in the permitted procfs overmount
   list. Its inclusion was almost certainly an error, and because `/proc/net`
   is a symlink to `/proc/self/net`, overmounting this was almost certainly
   never useful (and will be blocked by future kernel versions). (#4817, #4820)
 * CI: Switch to GitHub-hosted ARM runners. Thanks again to @alexellis for
   supporting runc's ARM CI up until now. (#4844, #4856, #4867)
 * Simplify the `prepareCriuRestoreMounts` logic for checkpoint-restore.
   (#4765, #4872)

## [1.2.6] - 2025-03-17

> Hasta la victoria, siempre.

### Fixed
 * Fix a stall issue that would happen if setting `O_CLOEXEC` with
   `CloseExecFrom` failed (#4647).
 * `runc` now properly handles joining time namespaces (such as with `runc
   exec`). Previously we would attempt to set the time offsets when joining,
   which would fail. (#4635, #4649)
 * Handle `EINTR` retries correctly for socket-related direct
   `golang.org/x/sys/unix` system calls. (#4650)
 * We no longer use `F_SEAL_FUTURE_WRITE` when sealing the runc binary, as it
   turns out this had some unfortunate bugs in older kernel versions and was
   never necessary in the first place. (#4651, #4640)

### Removed
 * Remove `Fexecve` helper from `libcontainer/system`. Runc 1.2.1 removed
   runc-dmz, but we forgot to remove this helper added only for that. (#4646)

###  Changed
 * Use Go 1.23 for official builds, run CI with Go 1.24 and drop Ubuntu 20.04
   from CI. We need to drop Ubuntu 20.04 from CI because Github Actions
   announced it's already deprecated and it will be discontinued soon. (#4648)

## [1.2.5] - 2025-02-13

> Мороз и солнце; день чудесный!

### Fixed
* There was a regression in systemd v230 which made the way we define device
  rule restrictions require a systemctl daemon-reload for our transient
  units. This caused issues for workloads using NVIDIA GPUs. Workaround the
  upstream regression by re-arranging how the unit properties are defined.
  (#4568, #4612, #4615)
 * Dependency github.com/cyphar/filepath-securejoin is updated to v0.4.1,
   allowing projects that vendor runc to bump it as well. (#4608)
 * CI: fixed criu-dev compilation. (#4611)

### Changed
 * Dependency golang.org/x/net is updated to 0.33.0. (#4632)

## [1.2.4] - 2025-01-07

> Христос се роди!

### Fixed
 * Re-add tun/tap devices to built-in allowed devices lists.

   In runc 1.2.0 we removed these devices from the default allow-list (which
   were added seemingly by accident early in Docker's history) as a precaution
   in order to try to reduce the attack surface of device inodes available to
   most containers (#3468). At the time we thought that the vast majority of
   users using tun/tap would already be specifying what devices they need (such
   as by using `--device` with Docker/Podman) as opposed to doing the `mknod`
   manually, and thus there would've been no user-visible change.

   Unfortunately, it seems that this regressed a noticeable number of users
   (and not all higher-level tools provide easy ways to specify devices to
   allow) and so this change needed to be reverted. Users that do not need
   these devices are recommended to explicitly disable them by adding deny
   rules in their container configuration. (#4555, #4556)

## [1.2.3] - 2024-12-12

> Winter is not a season, it's a celebration.

### Fixed
 * Fixed a regression in use of securejoin.MkdirAll, where multiple
   runc processes racing to create the same mountpoint in a shared rootfs
   would result in spurious EEXIST errors. In particular, this regression
   caused issues with BuildKit. (#4543, #4550)
 * Fixed a regression in eBPF support for pre-5.6 kernels after upgrading
   Cilium's eBPF library version to 0.16 in runc. (#3008, #4548, #4551)

## [1.2.2] - 2024-11-15

> Specialization is for insects.

### Fixed
 * Fixed the failure of `runc delete` on a rootless container with no
   dedicated cgroup on a system with read-only `/sys/fs/cgroup` mount.
   This is a regression in runc 1.2.0, causing a failure when using
   rootless buildkit. (#4518, #4531)
 * Using runc on a system where /run/runc and /usr/bin are on different
   filesystems no longer results in harmless but annoying messages
   ("overlayfs: "xino" feature enabled using 3 upper inode bits")
   appearing in the kernel log. (#4508, #4530)

### Changed
 * Better memfd-bind documentation. (#4530)
 * CI: bump Fedora 40 -> 41. (#4528)

## [1.2.1] - 2024-11-01

>  No existe una escuela que enseñe a vivir.

### Fixed
 * Became root after joining an existing user namespace. Otherwise, runc
   won't have permissions to configure some mounts when running under
   SELinux and runc is not creating the user namespace. (#4466, #4477)

### Removed
 * Remove dependency on `golang.org/x/sys/execabs` from go.mod. (#4480)
 * Remove runc-dmz, that had many limitations, and is mostly made obsolete by
   the new protection mechanism added in v1.2.0. Note that runc-dmz was only
   available only in the 1.2.0 release and required to set an environment variable
   to opt-in. (#4488)

### Added
 * The `script/check-config.sh` script now checks for overlayfs support. (#4494)
 * When using cgroups v2, allow to set or update memory limit to "unlimited"
   and swap limit to a specific value. (#4501)

## [1.2.0] - 2024-10-22

> できるときにできることをやるんだ。それが今だ。

### Added
 * In order to alleviate the remaining concerns around the memory usage and
   (arguably somewhat unimportant, but measurable) performance overhead of
   memfds for cloning `/proc/self/exe`, we have added a new protection using
   `overlayfs` that is used if you have enough privileges and the running
   kernel supports it. It has effectively no performance nor memory overhead
   (compared to no cloning at all). (#4448)

### Fixed
 * The original fix for [CVE-2024-45310][cve-2024-45310] was intentionally very
   limited in scope to make it easier to review, however it also did not handle
   all possible `os.MkdirAll` cases and thus could lead to regressions. We have
   switched to the more complete implementation in the newer versions of
   `github.com/cyphar/filepath-securejoin`. (#4393, #4400, #4421, #4430)
 * In certain situations (a system with lots of mounts or racing mounts) we
   could accidentally end up leaking mounts from the container into the host.
   This has been fixed. (#4417)
 * The fallback logic for `O_TMPFILE` clones of `/proc/self/exe` had a minor
   bug that would cause us to miss non-`noexec` directories and thus fail to
   start containers on some systems. (#4444)
 * Sometimes the cloned `/proc/self/exe` file descriptor could be placed in a
   way that it would get clobbered by the Go runtime. We had a fix for this
   already but it turns out it could still break in rare circumstances, but it
   has now been fixed. (#4294, #4452)

### Changed
 * It is not possible for `runc kill` to work properly in some specific
   configurations (such as rootless containers with no cgroups and a shared pid
   namespace). We now output a warning for such configurations. (#4398)
 * memfd-bind: update the documentation and make path handling with the systemd
   unit more idiomatic. (#4428)
 * We now use v0.16 of Cilium's eBPF library, including fixes that quite a few
   downstreams asked for. (#4397, #4396)
 * Some internal `runc init` synchronisation that was no longer necessary (due
   to the `/proc/self/exe` cloning move to Go) was removed. (#4441)

[cve-2024-45310]: https://github.com/opencontainers/runc/security/advisories/GHSA-jfvp-7x6p-h2pv

## [1.2.0-rc.3] - 2024-09-02

> The supreme happiness of life is the conviction that we are loved.

### Security

 * Fix [CVE-2024-45310][cve-2024-45310], a low-severity attack that allowed
   maliciously configured containers to create empty files and directories on
   the host.

### Added

 * Document build prerequisites for different platforms. (#4353)

### Fixed

 * Try to delete exec fifo file when failure in creation. (#4319)
 * Revert "libcontainer: seccomp: pass around *os.File for notifyfd". (#4337)
 * Fix link to gvariant documentation in systemd docs. (#4369)

### Changed

 * Remove pre-go1.17 build-tags. (#4329)
 * libct/userns: assorted (godoc) improvements. (#4330)
 * libct/userns: split userns detection from internal userns code. (#4331)
 * rootfs: consolidate mountpoint creation logic. (#4359)
 * Add Go 1.23, drop 1.21. (#4360)
 * Revert "allow overriding VERSION value in Makefile" and add `EXTRA_VERSION`.
   (#4370)
 * Mv contrib/cmd tests/cmd (except memfd-bind). (#4377)
 * Makefile: Don't read COMMIT, BUILDTAGS, `EXTRA_BUILDTAGS` from env vars.
   (#4380)

[cve-2024-45310]: https://github.com/opencontainers/runc/security/advisories/GHSA-jfvp-7x6p-h2pv

## [1.2.0-rc.2] - 2024-06-26

> TRUE or FALSE, it's a problem!

### Important Notes

 * libcontainer/cgroups users who want to manage cgroup devices need to explicitly
   import libcontainer/cgroups/devices. (#3452, #4248)
 * If building with Go 1.22.x, make sure to use 1.22.4 or a later version.
   (see #4233 for more details)

### Added

 * CI: add actuated-arm64. (#4142, #4252, #4276)

### Fixed

 * cgroup v2: do not set swap to 0 or unlimited when it's not available. (#4188)
 * Set the default value of CpuBurst to nil instead of 0. (#4210, #4211)
 * libct/cg: write unified resources line by line. (#4186)
 * libct.Start: fix locking, do not allow a second container init. (#4271)
 * Fix tests in debian testing (mount_sshfs.bats). (#4245)
 * Fix codespell warnings. (#4291)
 * libct/cg/dev: fix TestSetV1Allow panic. (#4295)
 * tests/int/scheduler: require smp. (#4298)

### Changed

 * libct/cg/fs: don't write cpu_burst twice on ENOENT. (#4259)
 * Make trimpath optional. (#3908)
 * Remove unused system.Execv. (#4268)
 * Stop blacklisting Go 1.22+, drop Go < 1.21 support, use Go 1.22 in CI. (#4292)
 * Improve some error messages for runc exec. (#4320)
 * ci/gha: bump golangci-lint[-action]. (#4255)
 * tests/int/tty: increase the timeout. (#4260)
 * [ci] use go mod instead of go get in spec.bats. (#4264)
 * tests/int/checkpoint: rm double logging. (#4251)
 * .cirrus.yml: rm FIXME from rootless fs on CentOS 7. (#4279)
 * Dockerfile: bump Debian to 12, Go to 1.21. (#4296)
 * ci/gha: switch to ubuntu 24.04. (#4286)
 * Vagrantfile.fedora: bump to F40. (#4285)

## [1.2.0-rc.1] - 2024-04-03

> There's a frood who really knows where his towel is.

`runc` now requires a minimum of Go 1.20 to compile.

> **NOTE**: runc currently will not work properly when compiled with Go 1.22 or
> newer. This is due to some unfortunate glibc behaviour that Go 1.22
> exacerbates in a way that results in containers not being able to start on
> some systems. [See this issue for more information.][runc-4233]

[runc-4233]: https://github.com/opencontainers/runc/issues/4233

### Breaking

 * Several aspects of how mount options work has been adjusted in a way that
   could theoretically break users that have very strange mount option strings.
   This was necessary to fix glaring issues in how mount options were being
   treated. The key changes are:

   - Mount options on bind-mounts that clear a mount flag are now always
     applied. Previously, if a user requested a bind-mount with only clearing
     options (such as `rw,exec,dev`) the options would be ignored and the
     original bind-mount options would be set. Unfortunately this also means
     that container configurations which specified only clearing mount options
     will now actually get what they asked for, which could break existing
     containers (though it seems unlikely that a user who requested a specific
     mount option would consider it "broken" to get the mount options they
     asked foruser who requested a specific mount option would consider it
     "broken" to get the mount options they asked for). This also allows us to
     silently add locked mount flags the user *did not explicitly request to be
     cleared* in rootless mode, allowing for easier use of bind-mounts for
     rootless containers. (#3967)

   - Container configurations using bind-mounts with superblock mount flags
     (i.e. filesystem-specific mount flags, referred to as "data" in
     `mount(2)`, as opposed to VFS generic mount flags like `MS_NODEV`) will
     now return an error. This is because superblock mount flags will also
     affect the host mount (as the superblock is shared when bind-mounting),
     which is obviously not acceptable. Previously, these flags were silently
     ignored so this change simply tells users that runc cannot fulfil their
     request rather than just ignoring it. (#3990)

   If any of these changes cause problems in real-world workloads, please [open
   an issue](https://github.com/opencontainers/runc/issues/new/choose) so we
   can adjust the behaviour to avoid compatibility issues.

### Added

 * runc has been updated to OCI runtime-spec 1.2.0, and supports all Linux
   features with a few minor exceptions. See
   [`docs/spec-conformance.md`](https://github.com/opencontainers/runc/blob/v1.2.0-rc.1/docs/spec-conformance.md)
   for more details.
 * runc now supports id-mapped mounts for bind-mounts (with no restrictions on
   the mapping used for each mount). Other mount types are not currently
   supported. This feature requires `MOUNT_ATTR_IDMAP` kernel support (Linux
   5.12 or newer) as well as kernel support for the underlying filesystem used
   for the bind-mount. See [`mount_setattr(2)`][mount_setattr.2] for a list of
   supported filesystems and other restrictions. (#3717, #3985, #3993)
 * Two new mechanisms for reducing the memory usage of our protections against
   [CVE-2019-5736][cve-2019-5736] have been introduced:
   - `runc-dmz` is a minimal binary (~8K) which acts as an additional execve
     stage, allowing us to only need to protect the smaller binary. It should
     be noted that there have been several compatibility issues reported with
     the usage of `runc-dmz` (namely related to capabilities and SELinux). As
     such, this mechanism is **opt-in** and can be enabled by running `runc`
     with the environment variable `RUNC_DMZ=true` (setting this environment
     variable in `config.json` will have no effect). This feature can be
     disabled at build time using the `runc_nodmz` build tag. (#3983, #3987)
   - `contrib/memfd-bind` is a helper daemon which will bind-mount a memfd copy
     of `/usr/bin/runc` on top of `/usr/bin/runc`. This entirely eliminates
     per-container copies of the binary, but requires care to ensure that
     upgrades to runc are handled properly, and requires a long-running daemon
     (unfortunately memfds cannot be bind-mounted directly and thus require a
     daemon to keep them alive). (#3987)
 * runc will now use `cgroup.kill` if available to kill all processes in a
   container (such as when doing `runc kill`). (#3135, #3825)
 * Add support for setting the umask for `runc exec`. (#3661)
 * libct/cg: support `SCHED_IDLE` for runc cgroupfs. (#3377)
 * checkpoint/restore: implement `--manage-cgroups-mode=ignore`. (#3546)
 * seccomp: refactor flags support; add flags to features, set `SPEC_ALLOW` by
   default. (#3588)
 * libct/cg/sd: use systemd v240+ new `MAJOR:*` syntax. (#3843)
 * Support CFS bandwidth burst for CPU. (#3749, #3145)
 * Support time namespaces. (#3876)
 * Reduce the `runc` binary size by ~11% by updating
   `github.com/checkpoint-restore/go-criu`. (#3652)
 * Add `--pidfd-socket` to `runc run` and `runc exec` to allow for management
   processes to receive a pidfd for the new process, allowing them to avoid pid
   reuse attacks. (#4045)

[mount_setattr.2]: https://man7.org/linux/man-pages/man2/mount_setattr.2.html
[cve-2019-5736]: https://github.com/advisories/GHSA-gxmr-w5mj-v8hh

### Deprecated

 * `runc` option `--criu` is now ignored (with a warning), and the option will
   be removed entirely in a future release. Users who need a non-standard
   `criu` binary should rely on the standard way of looking up binaries in
   `$PATH`. (#3316)
 * `runc kill` option `-a` is now deprecated. Previously, it had to be specified
   to kill a container (with SIGKILL) which does not have its own private PID
   namespace (so that runc would send SIGKILL to all processes). Now, this is
   done automatically. (#3864, #3825)
 * `github.com/opencontainers/runc/libcontainer/user` is now deprecated, please
   use `github.com/moby/sys/user` instead. It will be removed in a future
   release. (#4017)

### Changed

 * When Intel RDT feature is not available, its initialization is skipped,
   resulting in slightly faster `runc exec` and `runc run`. (#3306)
 * `runc features` is no longer experimental. (#3861)
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
 * libcontainer users that create and kill containers from a daemon process
   (so that the container init is a child of that process) must now implement
   a proper child reaper in case a container does not have its own private PID
   namespace, as documented in `container.Signal`. (#3825)
 * libcontainer: `container.Signal` no longer takes an `all` argument. Whether
   or not it is necessary to kill all processes in the container individually
   is now determined automatically. (#3825, #3885)
 * seccomp: enable seccomp binary tree optimization. (#3405)
 * `runc run`/`runc exec`: ignore SIGURG. (#3368)
 * Remove tun/tap from the default device allowlist. (#3468)
 * `runc --root non-existent-dir list` now reports an error for non-existent
   root directory. (#3374)

### Fixed

 * In case the runc binary resides on tmpfs, `runc init` no longer re-execs
   itself twice. (#3342)
 * Our seccomp `-ENOSYS` stub now correctly handles multiplexed syscalls on
   s390 and s390x. This solves the issue where syscalls the host kernel did not
   support would return `-EPERM` despite the existence of the `-ENOSYS` stub
   code (this was due to how s390x does syscall multiplexing). (#3474)
 * Remove tun/tap from the default device rules. (#3468)
 * specconv: avoid mapping "acl" to `MS_POSIXACL`. (#3739)
 * libcontainer: fix private PID namespace detection when killing the
   container. (#3866, #3825)
 * systemd socket notification: fix race where runc exited before systemd
   properly handled the `READY` notification. (#3291, #3293)
 * The `-ENOSYS` seccomp stub is now always generated for the native
   architecture that `runc` is running on. This is needed to work around some
   arguably specification-incompliant behaviour from Docker on architectures
   such as ppc64le, where the allowed architecture list is set to `null`. This
   ensures that we always generate at least one `-ENOSYS` stub for the native
   architecture even with these weird configs. (#4219)

### Removed

 * In order to fix performance issues in the "lightweight" bindfd protection
   against [CVE-2019-5736][cve-2019-5736], the temporary `ro` bind-mount of
   `/proc/self/exe` has been removed. runc now creates a binary copy in all
   cases. See the above notes about `memfd-bind` and `runc-dmz` as well as
   `contrib/cmd/memfd-bind/README.md` for more information about how this
   (minor) change in memory usage can be further reduced. (#3987, #3599, #2532,
   #3931)
 * libct/cg: Remove `EnterPid` (a function with no users). (#3797)
 * libcontainer: Remove `{Pre,Post}MountCmds` which were never used and are
   obsoleted by more generic container hooks. (#3350)

[cve-2019-5736]: https://github.com/advisories/GHSA-gxmr-w5mj-v8hh

## [1.1.15] - 2024-10-07

> How, dear sir, did you cross the flood? By not stopping, friend, and by not
> straining I crossed the flood.

### Fixed

 * The `-ENOSYS` seccomp stub is now always generated for the native
   architecture that `runc` is running on. This is needed to work around some
   arguably specification-incompliant behaviour from Docker on architectures
   such as ppc64le, where the allowed architecture list is set to `null`. This
   ensures that we always generate at least one `-ENOSYS` stub for the native
   architecture even with these weird configs. (#4391)
 * On a system with older kernel, reading `/proc/self/mountinfo` may skip some
   entries, as a consequence runc may not properly set mount propagation,
   causing container mounts leak onto the host mount namespace. (#2404, #4425)

### Removed

 * In order to fix performance issues in the "lightweight" bindfd protection
   against [CVE-2019-5736], the temporary `ro` bind-mount of `/proc/self/exe`
   has been removed. runc now creates a binary copy in all cases. (#4392, #2532)

[CVE-2019-5736]: https://www.openwall.com/lists/oss-security/2019/02/11/2

## [1.1.14] - 2024-09-03

> 年を取っていいことは、驚かなくなることね。

### Security

 * Fix [CVE-2024-45310][cve-2024-45310], a low-severity attack that allowed
   maliciously configured containers to create empty files and directories on
   the host.

[cve-2024-45310]: https://github.com/opencontainers/runc/security/advisories/GHSA-jfvp-7x6p-h2pv

### Added

 * Add support for Go 1.23. (#4360, #4372)

### Fixed

 * Revert "allow overriding VERSION value in Makefile" and add `EXTRA_VERSION`.
   (#4370, #4382)
 * rootfs: consolidate mountpoint creation logic. (#4359)

## [1.1.13] - 2024-06-13

> There is no certainty in the world. This is the only certainty I have.

### Important Notes

 * If building with Go 1.22.x, make sure to use 1.22.4 or a later version.
   (see #4233 for more details)

### Fixed

 * Support go 1.22.4+. (#4313)
 * runc list: fix race with runc delete. (#4231)
 * Fix set nofile rlimit error. (#4277, #4299)
 * libct/cg/fs: fix setting rt_period vs rt_runtime. (#4284)
 * Fix a debug msg for user ns in nsexec. (#4315)
 * script/*: fix gpg usage wrt keyboxd. (#4316)
 * CI fixes and misc backports. (#4241)
 * Fix codespell warnings. (#4300)

### Changed

 * Silence security false positives from golang/net. (#4244)
 * libcontainer: allow containers to make apps think fips is enabled/disabled for testing. (#4257)
 * allow overriding VERSION value in Makefile. (#4270)
 * Vagrantfile.fedora: bump Fedora to 39. (#4261)
 * ci/cirrus: rm centos stream 8. (#4305, #4308)

## [1.1.12] - 2024-01-31

> Now you're thinking with Portals™!

### Security

* Fix [CVE-2024-21626][cve-2024-21626], a container breakout attack that took
  advantage of a file descriptor that was leaked internally within runc (but
  never leaked to the container process). In addition to fixing the leak,
  several strict hardening measures were added to ensure that future internal
  leaks could not be used to break out in this manner again. Based on our
  research, while no other container runtime had a similar leak, none had any
  of the hardening steps we've introduced (and some runtimes would not check
  for any file descriptors that a calling process may have leaked to them,
  allowing for container breakouts due to basic user error).

[cve-2024-21626]: https://github.com/opencontainers/runc/security/advisories/GHSA-xr7r-f8xq-vfvv

## [1.1.11] - 2024-01-01

> Happy New Year!

### Fixed

* Fix several issues with userns path handling. (#4122, #4124, #4134, #4144)

### Changed

 * Support memory.peak and memory.swap.peak in cgroups v2.
   Add `swapOnlyUsage` in `MemoryStats`. This field reports swap-only usage.
   For cgroupv1, `Usage` and `Failcnt` are set by subtracting memory usage
   from memory+swap usage. For cgroupv2, `Usage`, `Limit`, and `MaxUsage`
   are set. (#4000, #4010, #4131)
 * build(deps): bump github.com/cyphar/filepath-securejoin. (#4140)

## [1.1.10] - 2023-10-31

> Śruba, przykręcona we śnie, nie zmieni sytuacji, jaka panuje na jawie.

### Added

* Support for `hugetlb.<pagesize>.rsvd` limiting and accounting. Fixes the
  issue of postres failing when hugepage limits are set. (#3859, #4077)

### Fixed

* Fixed permissions of a newly created directories to not depend on the value
  of umask in tmpcopyup feature implementation. (#3991, #4060)
* libcontainer: cgroup v1 GetStats now ignores missing `kmem.limit_in_bytes`
  (fixes the compatibility with Linux kernel 6.1+). (#4028)
* Fix a semi-arbitrary cgroup write bug when given a malicious hugetlb
  configuration. This issue is not a security issue because it requires a
  malicious `config.json`, which is outside of our threat model. (#4103)
* Various CI fixes. (#4081, #4055)

## [1.1.9] - 2023-08-10

> There is a crack in everything. That's how the light gets in.

### Added

* Added go 1.21 to the CI matrix; other CI updates. (#3976, #3958)

### Fixed

* Fixed losing sticky bit on tmpfs (a regression in 1.1.8). (#3952, #3961)
* intelrdt: fixed ignoring ClosID on some systems. (#3550, #3978)

### Changed

 * Sum `anon` and `file` from `memory.stat` for cgroupv2 root usage,
   as the root does not have `memory.current` for cgroupv2.
   This aligns cgroupv2 root usage more closely with cgroupv1 reporting.
   Additionally, report root swap usage as sum of swap and memory usage,
   aligned with v1 and existing non-root v2 reporting. (#3933)

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
[Unreleased]: https://github.com/opencontainers/runc/compare/v1.3.0-rc.1...HEAD
[1.3.0]: https://github.com/opencontainers/runc/compare/v1.3.0-rc.2...v1.3.0
[1.2.0]: https://github.com/opencontainers/runc/compare/v1.2.0-rc.1...v1.2.0
[1.1.0]: https://github.com/opencontainers/runc/compare/v1.1.0-rc.1...v1.1.0
[1.0.0]: https://github.com/opencontainers/runc/releases/tag/v1.0.0

<!-- 1.0.z patch releases -->
[Unreleased 1.0.z]: https://github.com/opencontainers/runc/compare/v1.0.3...release-1.0
[1.0.3]: https://github.com/opencontainers/runc/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/opencontainers/runc/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/opencontainers/runc/compare/v1.0.0...v1.0.1

<!-- 1.1.z patch releases -->
[Unreleased 1.1.z]: https://github.com/opencontainers/runc/compare/v1.1.15...release-1.1
[1.1.15]: https://github.com/opencontainers/runc/compare/v1.1.14...v1.1.15
[1.1.14]: https://github.com/opencontainers/runc/compare/v1.1.13...v1.1.14
[1.1.13]: https://github.com/opencontainers/runc/compare/v1.1.12...v1.1.13
[1.1.12]: https://github.com/opencontainers/runc/compare/v1.1.11...v1.1.12
[1.1.11]: https://github.com/opencontainers/runc/compare/v1.1.10...v1.1.11
[1.1.10]: https://github.com/opencontainers/runc/compare/v1.1.9...v1.1.10
[1.1.9]: https://github.com/opencontainers/runc/compare/v1.1.8...v1.1.9
[1.1.8]: https://github.com/opencontainers/runc/compare/v1.1.7...v1.1.8
[1.1.7]: https://github.com/opencontainers/runc/compare/v1.1.6...v1.1.7
[1.1.6]: https://github.com/opencontainers/runc/compare/v1.1.5...v1.1.6
[1.1.5]: https://github.com/opencontainers/runc/compare/v1.1.4...v1.1.5
[1.1.4]: https://github.com/opencontainers/runc/compare/v1.1.3...v1.1.4
[1.1.3]: https://github.com/opencontainers/runc/compare/v1.1.2...v1.1.3
[1.1.2]: https://github.com/opencontainers/runc/compare/v1.1.1...v1.1.2
[1.1.1]: https://github.com/opencontainers/runc/compare/v1.1.0...v1.1.1
[1.1.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.0.0...v1.1.0-rc.1

<!-- 1.2.z patch releases -->
[Unreleased 1.2.z]: https://github.com/opencontainers/runc/compare/v1.2.7...release-1.2
[1.2.7]: https://github.com/opencontainers/runc/compare/v1.2.6...v1.2.7
[1.2.6]: https://github.com/opencontainers/runc/compare/v1.2.5...v1.2.6
[1.2.5]: https://github.com/opencontainers/runc/compare/v1.2.4...v1.2.5
[1.2.4]: https://github.com/opencontainers/runc/compare/v1.2.3...v1.2.4
[1.2.3]: https://github.com/opencontainers/runc/compare/v1.2.2...v1.2.3
[1.2.2]: https://github.com/opencontainers/runc/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/opencontainers/runc/compare/v1.2.0...v1.2.1
[1.2.0-rc.3]: https://github.com/opencontainers/runc/compare/v1.2.0-rc.2...v1.2.0-rc.3
[1.2.0-rc.2]: https://github.com/opencontainers/runc/compare/v1.2.0-rc.1...v1.2.0-rc.2
[1.2.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.1.0...v1.2.0-rc.1

<!-- 1.3.z patch releases -->
[Unreleased 1.3.z]: https://github.com/opencontainers/runc/compare/v1.3.2...release-1.3
[1.3.2]: https://github.com/opencontainers/runc/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/opencontainers/runc/compare/v1.3.0...v1.3.1
[1.3.0-rc.2]: https://github.com/opencontainers/runc/compare/v1.3.0-rc.1...v1.3.0-rc.2
[1.3.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.2.0...v1.3.0-rc.1

<!-- 1.4.z patch releases -->
[1.4.0-rc.1]: https://github.com/opencontainers/runc/compare/v1.3.0...v1.4.0-rc.1

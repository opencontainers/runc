# runc threat model

This document describes what runc considers a security vulnerability. It
expands on [SECURITY.md](SECURITY.md); please read it before reporting a
security issue.

## What runc is

runc is the reference implementation of the [OCI Runtime
Specification](https://github.com/opencontainers/runtime-spec). It is a
low-level container runtime: given an OCI runtime configuration
(`config.json`) and a root file system, it creates and configures the Linux
isolation primitives (namespaces, cgroups, capabilities, seccomp, devices,
mounts, `pivot_root`, AppArmor and SELinux labels) and then executes the
container process. Its core is also available as a Go library
(`libcontainer`).

runc is not a container engine or daemon (Docker, containerd, CRI-O, Podman),
an image builder or puller, an orchestrator, a hypervisor, or a network or
storage manager. It is invoked by an engine, as root or as an unprivileged user
(rootless).

## Core principle

runc is responsible for correctly enforcing the isolation that the OCI
configuration asks for. If runc implements what `config.json` requests and the
container behaves as configured, there is no runc vulnerability, even if the
configuration is insecure. A vulnerability exists when untrusted content makes
runc's actual behavior weaker than the isolation it was asked to provide.

runc does not sanitize, harden or second-guess the configuration. For example:

- The set of masked and read-only paths is defined entirely by the
  configuration. If it omits `/proc/kcore` or `/proc/sysrq-trigger`, the
  container can access them.
- Mounts are performed in the order they are listed, and a later mount
  shadows an earlier one. If the configuration lists a mount on top of
  `/proc`, `/sys` or `/dev`, the container gets what it asked for. Masked and
  read-only paths are applied after all mounts.

## Assets

- The host file system, processes and kernel state outside the container.
- The privileges of the user running runc (root or rootless).
- Other containers running on the same host.
- The container engine and other host processes that interact with the
  container, including later runc invocations (`exec`, `ps`, `kill`, `update`,
  `delete`).
- The integrity of the security policy requested for the container
  (namespaces, capabilities, seccomp, LSM labels, device rules, cgroup limits,
  masked and read-only paths, `noNewPrivileges`).

## Trust boundaries

| Input | Trust level | Notes |
|-------|-------------|-------|
| OCI runtime configuration (`config.json`) | **Trusted**, with exceptions (see "Configuration" below) | Provided by the container engine, which is responsible for validating user input. This includes hooks, which run on the host. |
| Command line, global options and environment (`--root`, `--log`, `--criu`, `--systemd-cgroup`, `--no-pivot`, `runc exec --process`, console socket, pid file, checkpoint image directory) | **Trusted** | Provided by the engine/operator. |
| Annotations listed under `potentiallyUnsafeConfigAnnotations` in `runc features` (`bundle`, `org.systemd.property.*`, `org.criu.config`) | **Trusted** | Callers must validate them. |
| Container state directory (`--root`, `/run/runc` by default) | **Trusted** | Host-side, not accessible to the container. |
| Callers of the `libcontainer` Go API | **Trusted** | |
| Container rootfs contents (binaries, libraries, symlinks, device nodes, mount points) | **Untrusted** | Primary attacker-controlled input. |
| Contents of volumes and other bind-mount sources the container can write to, including those shared with other containers | **Untrusted** | Can be modified concurrently while runc sets up another container. |
| Processes running inside the container | **Untrusted** | They can race runc during `run`, `create` and `exec`. |
| Files read by runc from the rootfs (`/etc/passwd`, `/etc/group`) | **Untrusted** | |
| What runc sees through the container's `/proc`, `/dev`, `/sys` and cgroup files the container can write to (delegated or writable cgroup hierarchy) | **Untrusted** | The container may replace or shadow them. |
| Bundle files other than `config.json` and the rootfs | **Trusted** | Provided by the engine. |

### Configuration

runc does not check `config.json` against any policy: it does what the
configuration asks for. Engines, however, often fill some of its fields from
input of a user who is not trusted with the host (an image config, a
Kubernetes pod spec). The fields fall into two groups.

Fields that define the isolation and privileges of the container. An untrusted
user must never control them; if an engine allows that, it is a bug in the
engine, not in runc:

- mount sources, types and options (the bind-mount source path is trusted,
  even though the contents of a writable volume are not);
- the rootfs path; the rootfs must be in a directory that only root or the
  container's owner can access, as containerd, CRI-O, umoci and others do;
- devices, device cgroup rules, cgroup path and resource limits;
- capabilities, seccomp, AppArmor and SELinux labels, `noNewPrivileges`,
  masked and read-only paths;
- namespace paths and user namespace mappings;
- sysctls (engines must only allow safe ones);
- hooks and the annotations listed above.

Fields that only describe what runs inside the container: process arguments,
environment, working directory, uid and gid, hostname, and mount destinations.
Engines commonly take them from untrusted input (`ENTRYPOINT`, `ENV`,
`WORKDIR` and `USER` in an image; `command`, `env`, `workingDir` and
`mountPath` in a pod spec). runc does not validate them and does not need to:
they cannot give the container more than the first group allows. If some value
of these fields makes runc break the requested isolation, it is a runc
vulnerability. For example, CVE-2024-21626 could be exploited by setting the
working directory to `/proc/self/fd/<N>`, which pointed to a leaked host
directory; and a mount destination must be resolved inside the rootfs even if
it contains `..` or goes through a symlink in the rootfs.

Fields added to the runtime spec in the future are in the first group unless
this document says otherwise.

## Attacker model

The attacker controls the contents of a container image/rootfs, the contents
of volumes the container can write to, and the code running inside the
container, but does not control the trusted parts of `config.json` (see
"Configuration"), runc's command line, or the host. The attacker's goals are
to:

1. Escape the container (read or write host files, access host processes or
   namespaces, execute code on the host).
2. Escalate privileges (gain capabilities, bypass user namespace mappings,
   gain root on the host, or, for rootless runc, gain privileges the invoking
   user does not have).
3. Bypass the security policy configured for the container.
4. Compromise other containers, the container engine, or other host processes
   interacting with the container.

## In scope (vulnerabilities)

- Symlink, bind-mount, `..` traversal, or mount-point tricks in the rootfs or
  in volumes that make runc read, write, mount, chmod/chown or create files
  outside the rootfs, including TOCTOU races (mount destination handling,
  masked and read-only paths, `/dev` and `/dev/console` setup, procfs access).
- Rootfs content or container processes causing runc to apply an incorrect or
  weaker security policy than the one requested (seccomp filter, capabilities,
  LSM label, device rules, masked/read-only paths, `noNewPrivileges`,
  cgroup limits).
- Leaking host file descriptors, host mounts, host namespaces or host
  environment into the container process, or leaving the container process
  able to reach them.
- Races between runc and container processes that let the container affect
  runc's behavior: `runc init` before the container process is executed,
  `runc exec` joining a running container, `runc update`, `runc ps` and other
  commands reading `/proc/<pid>` or cgroup files, checkpoint and restore.
  This assumes the container process has no capabilities that let it override
  runc's protections (see "Out of scope").
- Container escape through runc's own binary, e.g. overwriting it via
  `/proc/self/exe` (see CVE-2019-5736).
- Privilege escalation in the rootless path (user namespace setup,
  `newuidmap`/`newgidmap` interaction, cgroup and `/sys` access).
- Bugs in parsing files from the rootfs (`/etc/passwd`, `/etc/group`) that
  lead to wrong user, group or privilege settings, and memory-safety bugs in
  runc's C code (`libcontainer/nsenter`) reachable from untrusted input.

Examples of past runc vulnerabilities:

| CVE | Summary |
|-----|---------|
| [CVE-2019-5736](https://github.com/advisories/GHSA-gxmr-w5mj-v8hh) | Container process could overwrite the host runc binary via `/proc/self/exe`. |
| [CVE-2021-30465](https://github.com/opencontainers/runc/security/advisories/GHSA-c3xm-pvg7-gh7r) | Mount destinations could be swapped via symlink-exchange to mount outside the rootfs. |
| [CVE-2023-28642](https://github.com/opencontainers/runc/security/advisories/GHSA-g2j6-57v7-gm8c) | AppArmor/SELinux bypass with a symlinked `/proc` in the image. |
| [CVE-2024-21626](https://github.com/opencontainers/runc/security/advisories/GHSA-xr7r-f8xq-vfvv) | Internally leaked host file descriptor led to container breakouts. |
| [CVE-2025-31133](https://github.com/opencontainers/runc/security/advisories/GHSA-9493-h29p-rfm2) | Container escape via masked path abuse due to mount race conditions. |
| [CVE-2025-52881](https://github.com/opencontainers/runc/security/advisories/GHSA-cgrx-mc8f-2prm) | Container escape and denial of service due to arbitrary write gadgets and procfs write redirects. |

In every case above, the container was not given special privileges; the
isolation broke because of a defect in runc itself.

## Out of scope (not vulnerabilities)

- Anything that requires a malicious or crafted value in a trusted part of
  `config.json` (see "Configuration"), e.g. mounting host paths (`/`,
  `/var/run/docker.sock`, `/proc`, `/sys`, `/dev`), granting dangerous
  capabilities (`CAP_SYS_ADMIN`, `CAP_SYS_PTRACE`, ...) or all of them,
  disabling seccomp, not using a user namespace, using host namespaces, or
  omitting masked and read-only paths. These are explicit configuration
  requests; report them to the engine or tool that generated the
  configuration.
- Attacks on `runc init` or `runc exec` that rely on capabilities granted by
  the configuration, such as `CAP_SYS_PTRACE` without a user namespace:
  joining a container is not atomic, and such a process can ptrace or access
  `/proc/<pid>/mem` of a non-dumpable `runc init`.
- Misuse of `potentiallyUnsafeConfigAnnotations` or of options documented as
  unsafe (such as `--no-pivot`); validating them is the responsibility of the
  caller.
- Bugs that need the attacker to already control the host, the container
  engine, runc's command line or environment, the state directory, or the
  runc binary itself.
- Denial of service by a container against itself, or resource exhaustion that
  is bounded by the cgroup limits the engine requested. If runc fails to apply
  limits that are present in the configuration, that is in scope.
- Kernel vulnerabilities or weaknesses in kernel isolation primitives
  (namespaces, cgroups, seccomp, LSMs, overlayfs) that runc merely uses. Report
  these to the [Linux kernel security
  team](https://www.kernel.org/doc/html/latest/process/security-bugs.html).
- The shared-kernel model of containers: containers run on the host kernel by
  design, and stronger isolation needs a hypervisor-based runtime.
- The content of container images (malicious binaries, vulnerable libraries,
  leaked secrets). runc does not build, pull or scan images; only a rootfs
  that tricks runc itself is in scope.
- Vulnerabilities in CRIU, libseccomp, systemd, the Go runtime or other
  external components; these should be reported to the respective projects
  unless runc misuses them.
- Issues only reproducible in unsupported or end-of-life releases (see
  "Support Policy" in [RELEASES.md](RELEASES.md)).
- Findings in tests, CI configuration, documentation, fuzzing harnesses and
  scripts (`tests/`, `script/`, `contrib/`, `.github/`) that do not affect the
  shipped runc binary. Supply-chain concerns about the release process are
  welcome but are tracked as ordinary issues.

## How to exercise it

See [README.md](README.md) for build dependencies, build tags, running the
test suite and creating a bundle by hand. In addition:

- `make localunittest`, `sudo make localintegration` and
  `make localrootlessintegration` run the tests on the host instead of in a
  container; `TESTPATH=/mounts.bats` limits the integration tests to a single
  file.
- The integration tests download busybox and Debian images
  (`tests/integration/get-images.sh`); see `setup_bundle` in
  `tests/integration/helpers.bash` for how a bundle is generated. The
  `spec validator` test in `spec.bats` needs network access (it clones
  runtime-spec), so it fails when run offline.
- Use `runc exec <id> <cmd>` on a running container to exercise the `exec`
  path.
- Only the rootfs content, volume content and the code running in the
  container are attacker controlled. A realistic proof of concept builds a
  malicious image (for example one containing symlinks, odd mount points or
  device nodes), runs it through a container engine configured to use the
  runc under test (e.g. `podman --runtime /path/to/runc` or a Docker/containerd
  runtime configuration), and shows a read, write or privilege effect on the
  host. Races against `runc exec` or mount setup may need a process inside the
  container, or a second container sharing a volume, to win them. Running runc
  by hand, as described in README.md, is fine for development, but it does not
  by itself demonstrate a vulnerability; see "Reporting".

## How to rate severity

The final severity (CVSS score) is assigned by the maintainers. As a rough
guide:

- Critical: a container escape, host file write, or code execution on the host
  from an unprivileged container with a default-like configuration, using only
  attacker-controlled image content, with no special conditions.
- High: the same, but requiring conditions such as winning a race or a shared
  volume between containers (e.g. CVE-2021-30465, CVE-2024-21626,
  CVE-2025-31133); or a bypass of seccomp, capabilities or
  `noNewPrivileges`.
- Medium: a bypass of other requested policy (LSM, masked or read-only paths)
  without a demonstrated escape (e.g. CVE-2023-28642); an information leak from
  the host or another container; a denial of service against the host or the
  container engine that crosses cgroup limits.
- Low: limited host file system effects such as creating empty files (e.g.
  CVE-2024-45310), hardening issues, and bugs that need unusual but still
  trusted configuration.
- Anything that needs control of the trusted parts of `config.json`, the
  annotations marked unsafe, or the command line is out of scope (see above),
  whatever the impact.

## Anything to leave alone

- Do not report behavior that follows directly from what `config.json` asks
  for (host mounts, added capabilities, disabled seccomp, host namespaces,
  missing masked paths).
- Do not report build, packaging, CI, test or documentation issues.
- Do not report problems in the kernel, CRIU, libseccomp, systemd or other
  external components unless runc uses them incorrectly.
- Do not report a crash or misbehavior caused by a trusted caller passing
  invalid arguments to the `libcontainer` Go API.

## Reporting

The proof of concept must use at least one high-level container runtime or
engine (for example Docker, containerd, CRI-O, Podman or Kubernetes) and must
not invoke runc directly. When runc is invoked directly, the reporter controls
all of `config.json`, including its trusted parts, so the issue cannot be told
apart from an explicit configuration request. If an issue can only be
reproduced by running runc directly, it is unlikely it is a runc
vulnerability.

Before reporting, check that:

- the container does not run with dangerous capabilities, as privileged, or
  with host namespaces, unless the issue does not depend on that;
- the behavior does not come from a host path mounted into the container;
- the behavior is not the direct consequence of a configuration option;
- the root cause is not in the engine, the orchestrator, the image, or the
  kernel;
- runc applies weaker isolation than the configuration specifies. If so, this
  is a runc vulnerability; please report it.

Prefer a short reproducer and, if possible, a small patch.

Report suspected vulnerabilities privately, following the process described
in [SECURITY.md](SECURITY.md). Do not open a public issue or pull request.

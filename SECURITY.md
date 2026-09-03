# Security

When reporting a security issue, do not create an issue or file a pull request on GitHub.
The reporting process and disclosure communications are outlined [here](https://github.com/opencontainers/org/blob/master/SECURITY.md).
Before reporting, please read runc's threat model definition.

## Threat Model

### What runc Is

[runc](https://github.com/opencontainers/runc) is the reference
implementation of the
[OCI Runtime Specification](https://github.com/opencontainers/runtime-spec).
It is a **low-level container runtime**: given an OCI runtime
configuration (`config.json`) and a root filesystem, runc creates and
configures the Linux isolation primitives (namespaces, cgroups,
capabilities, seccomp, devices, mounts, rootfs) and then starts the
containerized process.

runc is **not**:

- A container engine or daemon -- that is [containerd](https://containerd.io),
  [Docker](https://docker.com), [CRI-O](https://cri-o.io), etc.
- An image builder or image puller -- that is BuildKit, Docker, containerd,
  Skopeo, etc.
- A container orchestrator -- that is Kubernetes, Docker swarm, etc.
- A hypervisor -- Linux containers share the host kernel by design.
- A network or storage manager.

### The Security Boundary runc Provides

runc's responsibility is to **correctly and securely implement the
isolation primitives defined by the OCI Runtime Specification**. This
includes:

- **Linux namespaces** -- mount, PID, network, IPC, UTS, user, cgroup, time.
- **Control groups (cgroups)** -- resource limits and device access control.
- **Linux capabilities** -- dropping, adding, or bounding capabilities per the spec.
- **Seccomp** -- applying the syscall filter described in the spec.
- **Devices** -- allow/deny device nodes per the spec.
- **Root filesystem isolation** -- `pivot_root`, mount propagation, masked/readonly paths.
- **MAC** -- applying AppArmor and SELinux labels per the spec.
- **`noNewPrivileges`** and related hardening flags.

**The core principle:**

> runc is responsible for correctly enforcing the isolation that the OCI
> runtime configuration *asks for*. If runc correctly implements the spec
> and the container behaves exactly as the configuration specifies -- even
> if that configuration is insecure -- there is **NO runc vulnerability**.
> A runc vulnerability exists **only when runc's actual behavior is weaker
> than the isolation the specification says it should provide.**

In other words, if a container escape or privilege gain is achievable only
because you can control `config.json`, that is **NOT** a runc vulnerability.

Therefore, your `PoC` must use at least one high-level container runtime
(e.g., Docker, containerd, CRI-O, or Kubernetes) and must not invoke runc directly.

### Trust Boundaries and Assumptions

runc operates on the OCI runtime configuration (`config.json`) provided
by an upstream container engine -- such as containerd, CRI-O, or Docker.
runc **trusts** this configuration: it assumes the upstream has generated
a specification that correctly describes the desired isolation posture,
and runc's role is to faithfully implement it.

This trust relationship has concrete implications:

**runc does not sanitize or override the configuration.** If the upstream
specifies weak security settings -- missing `maskedPaths`, incomplete
`readOnlyPaths`, excessive capabilities, or dangerous mounts -- runc will
apply those settings as written. runc is a runtime that executes the
policy defined in the configuration; it is not a policy engine that
judges or hardens that policy.

**Mount ordering.** runc processes mounts in the order given by the OCI
configuration. There is an established convention that critical system
directories -- `/proc`, `/sys`, `/dev` -- should be mounted and secured
*before* any user-defined bind mounts are applied. If the upstream engine
produces a configuration where custom mounts precede these system directories,
a custom mount may shadow or override a system mount point, potentially
weakening isolation. This is a configuration-generation issue in the upstream
engine, not a runc bug.

**`maskedPaths` and `readOnlyPaths`.** The set of paths to mask or make
read-only is defined entirely by the upstream engine in the OCI
configuration, not by runc. runc applies exactly the paths listed -- no
more, no fewer. If the upstream omits critical paths (for example,
failing to mask `/proc/kcore` or `/proc/sysrq-trigger`), and an attacker
inside the container can access those paths as a result, that gap is the
upstream's responsibility. runc faithfully applied the list it was given.

### In Scope -- These Are runc Vulnerabilities

A defect in runc's own code or logic that causes the actual security
posture of a container to be **weaker** than what the OCI runtime
specification (and runc's own documentation) says it should be.

In concrete terms, this includes:

- A process inside a **properly configured** container being able to
  access host resources, escape the container, or escalate privileges
  beyond what the configuration allows -- due to a bug in runc's
  implementation of namespaces, cgroups, capabilities, seccomp, mounts,
  devices, or rootfs handling.
- runc leaking host-side resources (file descriptors, mounts, library
  paths, environment variables) into the container in a way that undermines
  isolation.
- runc applying a weaker seccomp / capability / device / MAC policy than
  the one specified in the config.
- A **malicious or crafted OCI image** compromising the host **during
  container creation** through a bug in runc's handling of the rootfs,
  mounts, or spec parsing.
- runc-init or any runc helper process executing attacker-controlled code
  on the host due to a logic error.
- Race conditions in runc's setup path that break isolation guarantees.

**Real-world examples of runc CVEs (for reference):**

| CVE | Summary | Why it is a runc bug |
|-----|---------|----------------------|
| [CVE-2019-5736](https://github.com/advisories/GHSA-gxmr-w5mj-v8hh) | Container process could overwrite the runc binary on the host via `/proc/self/exe` | runc allowed the container to write to the host runc binary |
| [CVE-2024-21626](https://github.com/advisories/GHSA-xr7r-f8xq-vfvv) | File descriptor to a host directory leaked into the container process | runc leaked a host fd, breaking isolation |
| [CVE-2025-52881](https://github.com/advisories/GHSA-cgrx-mc8f-2prm) | Container escape / DoS via procfs write redirection (arbitrary write gadgets) | runc misdirected procfs writes, breaking isolation |

In every case above, the container was **not** given special privileges --
the isolation broke because of a defect in runc itself.

### Out of Scope -- These Are NOT runc Vulnerabilities

#### 1. Linux kernel vulnerabilities

runc relies on the kernel to enforce isolation once it is configured.
If the kernel's implementation of namespaces, cgroups, seccomp, BPF,
or capabilities is buggy, that is a **kernel vulnerability**.

Examples that are **not** runc issues:

- A kernel bug that allows escaping a user namespace.
- A kernel bug in cgroup v1/v2 device controller that bypasses device
  restrictions.
- A kernel bug that allows bypassing seccomp filters.
- A kernel bug in `overlayfs`, `pivot_root`, or mount propagation.
- Any "container escape" whose root cause is a kernel CVE.

Report these to the
[Linux kernel security team](https://www.kernel.org/doc/html/latest/process/security-bugs.html),
**not** to runc.

#### 2. Insecure configurations explicitly requested by the operator

If an operator configures a container with settings that are documented
as dangerous, and the container then behaves dangerously, **runc did
exactly what it was told to do.** This is not a bug.

Examples that are **not** runc issues:

- Granting **dangerous capabilities** -- `CAP_SYS_ADMIN`,
  `CAP_SYS_PTRACE`, `CAP_SYS_MODULE`, `CAP_DAC_READ_SEARCH`, etc. These
  capabilities are documented as powerful; using them with untrusted
  workloads is the operator's choice.
- Running a container with **all capabilities** (the `--privileged`
  equivalent in the OCI spec).
- **Bind-mounting host paths** into the container -- especially `/`,
  `/var/run/docker.sock`, `/proc`, `/sys`, `/dev`, or the host's home
  directory. If you give the container access to host files, it has
  access to host files.
- **Disabling seccomp** (`seccomp` disabled in the spec).
- Using **host namespaces** -- `hostNetwork`, `hostPID`, `hostIPC`, or no
  user namespace.

**The rule of thumb:** if the behavior follows directly from a
configuration option the operator explicitly set, and runc applied that
option correctly, there is no runc vulnerability -- regardless of how
insecure the outcome is. Report the *misconfiguration* to the operator or
the higher-level tool that generated the configuration, not to runc.

#### 3. The fundamental shared-kernel model of containers

Linux containers **share the host kernel**. This is a fundamental design
property, not a defect. A containerized process runs syscalls on the
host kernel; if that kernel is exploitable, the container can attack it.
This is inherent to the container model.

If you need stronger isolation than the shared-kernel model provides, use
a hypervisor-based isolation layer such as
[Kata Containers](https://katacontainers.io), Firecracker, or a
hardware-isolated runtime. The absence of a hypervisor boundary in runc
is **by design** and is not a vulnerability.

#### 4. Resource exhaustion / denial of service

If a container is configured with resource limits (cgroup memory/CPU/IO
limits) and those limits are enforced correctly, but the container still
causes problems (e.g., fork bombs hitting PID limits, disk fill, noisy
neighbor), the root cause is typically the **cgroup or kernel**, or the
operator's choice of limits -- not runc.

If runc fails to *apply* the resource limits specified in the OCI config
(that is, the limits are present in the spec but not actually enforced),
that **would** be a runc issue.

#### 5. Container image content / supply chain

runc does not build, pull, scan, or trust container images. If an image
contains malicious binaries, vulnerable libraries, or leaked secrets,
that is an **image supply-chain** issue. Report it to the image publisher
or the tooling that produced the image.

#### 6. Features documented as unsafe for untrusted workloads

Some runc features or OCI spec options are explicitly documented as not
safe for use with untrusted containers. If an operator uses such a feature
with untrusted input and it behaves as documented, that is not a
vulnerability -- it is documented behavior.

For example, the annotations flagged as potentially unsafe, which can be
found in the output of `runc features`, under the key
`potentiallyUnsafeConfigAnnotations`.

### Before You Report

Please work through this checklist **before** filing a report:

1. **Is the container running with dangerous capabilities
   (`CAP_SYS_ADMIN`, `CAP_SYS_PTRACE`, …), or privileged?**
   If yes, and the behavior stems from those privileges, it is expected.

2. **Did you mount any host path into the container?**
   If yes, the container has access to those host paths by design.

3. **Can you reproduce the issue using at least one high-level container runtime?**
   If the issue can only be reproduced when using runc directly, then this is not a vulnerability in runc.

4. **Is the behavior the *direct* consequence of a configuration option
   you explicitly set?**
   If so, runc is following your instructions.

5. **Is the root cause in Docker, containerd, CRI-O, Kubernetes, or an
   image builder?**
   If so, report to that project.

6. **Does runc apply a weaker isolation than the OCI config specifies?**
   If **yes** -- that is a runc vulnerability. Please report it.

If, after this checklist, you still believe you have found a runc
vulnerability, we genuinely want to hear from you. Follow [the reporting
process](https://github.com/opencontainers/org/blob/master/SECURITY.md) linked at the top of this document.

---

## Acknowledgements

We appreciate the security research community's continued engagement
with runc. We hope that by clarifying the threat model, this document
helps make that collaboration as productive as possible -- helping
researchers quickly identify whether a finding falls within runc's scope
so we can respond faster and more effectively.

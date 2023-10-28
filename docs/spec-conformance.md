# Spec conformance

This branch of runc implements the [OCI Runtime Spec v1.1.0](https://github.com/opencontainers/runtime-spec/tree/v1.1.0)
for the `linux` platform.

The following features are not implemented yet:

Spec version | Feature                                  | PR
-------------|------------------------------------------|----------------------------------------------------------
v1.1.0       | `SECCOMP_FILTER_FLAG_WAIT_KILLABLE_RECV` | [#3862](https://github.com/opencontainers/runc/pull/3862)
v1.1.0       | rsvd hugetlb cgroup                      | TODO ([#3859](https://github.com/opencontainers/runc/issues/3859))
v1.1.0       | `.process.ioPriority`                    | [#3783](https://github.com/opencontainers/runc/pull/3783)


The following features are implemented with some limitations:
Spec version | Feature                                  | Limitation
-------------|------------------------------------------|----------------------------------------------------------
v1.1.0       | `.[]mounts.uidMappings`                  | Requires using UserNS with identical uidMappings
v1.1.0       | `.[]mounts.gidMappings`                  | Requires using UserNS with identical gidMappings

## Architectures

The following architectures are supported:

runc binary  | seccomp
-------------|-------------------------------------------------------
`amd64`      | `SCMP_ARCH_X86`, `SCMP_ARCH_X86_64`, `SCMP_ARCH_X32`
`arm64`      | `SCMP_ARCH_ARM`, `SCMP_ARCH_AARCH64`
`armel`      | `SCMP_ARCH_ARM`
`armhf`      | `SCMP_ARCH_ARM`
`ppc64le`    | `SCMP_ARCH_PPC64LE`
`riscv64`    | `SCMP_ARCH_RISCV64`
`s390x`      | `SCMP_ARCH_S390`, `SCMP_ARCH_S390X`

The runc binary might be compilable for i386, big-endian PPC64,
and several MIPS variants too, but these architectures are not officially supported.

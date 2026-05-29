# Spec conformance

This branch of runc implements the [OCI Runtime Spec v1.3.0](https://github.com/opencontainers/runtime-spec/tree/v1.3.0)
for the `linux` platform.

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
`loong64`    | `SCMP_ARCH_LOONGARCH64`

The runc binary might be compilable for i386, big-endian PPC64,
and several MIPS variants too, but these architectures are not officially supported.

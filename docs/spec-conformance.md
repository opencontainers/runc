# Spec conformance

This branch of runc implements the [OCI Runtime Spec v1.1.0-rc.1](https://github.com/opencontainers/runtime-spec/tree/v1.1.0-rc.1)
for the `linux` platform.

The following features are not implemented yet:

Spec version | Feature                                  | PR
-------------|------------------------------------------|----------------------------------------------------------
v1.0.0       | `SCMP_ARCH_PARISC`                       | Unplanned, due to lack of users
v1.0.0       | `SCMP_ARCH_PARISC64`                     | Unplanned, due to lack of users
v1.0.2       | `.linux.personality`                     | [#3126](https://github.com/opencontainers/runc/pull/3126)
v1.1.0-rc.1  | `.linux.resources.cpu.burst`             | [#3205](https://github.com/opencontainers/runc/pull/3205)
v1.1.0-rc.1  | `.domainname`                            | [#3600](https://github.com/opencontainers/runc/pull/3600)
v1.1.0-rc.1  | `.[]mounts.uidMappings`                  | [#3717](https://github.com/opencontainers/runc/pull/3717)
v1.1.0-rc.1  | `.[]mounts.gidMappings`                  | [#3717](https://github.com/opencontainers/runc/pull/3717)
v1.1.0-rc.1  | `SECCOMP_FILTER_FLAG_WAIT_KILLABLE_RECV` | TODO

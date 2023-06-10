# Spec conformance

This branch of runc implements the [OCI Runtime Spec v1.1.0-rc.3](https://github.com/opencontainers/runtime-spec/tree/v1.1.0-rc.3)
for the `linux` platform.

The following features are not implemented yet:

Spec version | Feature                                  | PR
-------------|------------------------------------------|----------------------------------------------------------
v1.0.0       | `SCMP_ARCH_PARISC`                       | Unplanned, due to lack of users
v1.0.0       | `SCMP_ARCH_PARISC64`                     | Unplanned, due to lack of users
v1.0.2       | `.linux.personality`                     | [#3126](https://github.com/opencontainers/runc/pull/3126)
v1.1.0-rc.1  | `.linux.resources.cpu.burst`             | [#3749](https://github.com/opencontainers/runc/pull/3749)
v1.1.0-rc.1  | `.[]mounts.uidMappings`                  | [#3717](https://github.com/opencontainers/runc/pull/3717)
v1.1.0-rc.1  | `.[]mounts.gidMappings`                  | [#3717](https://github.com/opencontainers/runc/pull/3717)
v1.1.0-rc.1  | `SECCOMP_FILTER_FLAG_WAIT_KILLABLE_RECV` | [#3862](https://github.com/opencontainers/runc/pull/3862)
v1.1.0-rc.2  | time namespaces                          | [#3876](https://github.com/opencontainers/runc/pull/3876)
v1.1.0-rc.2  | rsvd hugetlb cgroup                      | TODO ([#3859](https://github.com/opencontainers/runc/issues/3859))
v1.1.0-rc.3  | `.process.scheduler`                     | TODO ([#3895](https://github.com/opencontainers/runc/issues/3895))
v1.1.0-rc.3  | `.process.ioPriority`                    | [#3783](https://github.com/opencontainers/runc/pull/3783)

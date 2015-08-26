# Linux-specific configuration

The Linux container specification uses various kernel features like namespaces,
cgroups, capabilities, LSM, and file system jails to fulfill the spec.
Additional information is needed for Linux over the [default spec configuration](config.md)
in order to configure these various kernel features.

## Capabilities

Capabilities is an array that specifies Linux capabilities that can be provided to the process
inside the container. Valid values are the string after `CAP_` for capabilities defined
in [the man page](http://man7.org/linux/man-pages/man7/capabilities.7.html)

```json
   "capabilities": [
        "AUDIT_WRITE",
        "KILL",
        "NET_BIND_SERVICE"
    ]
```

## Rootfs Mount Propagation

rootfsPropagation sets the rootfs's mount propagation. Its value is either slave, private, or shared. [The kernel doc](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt) has more information about mount propagation.

```json
    "rootfsPropagation": "slave",
```

## User namespace mappings

```json
    "uidMappings": [
        {
            "hostID": 1000,
            "containerID": 0,
            "size": 10
        }
    ],
    "gidMappings": [
        {
            "hostID": 1000,
            "containerID": 0,
            "size": 10
        }
    ]
```

uid/gid mappings describe the user namespace mappings from the host to the container.
The mappings represent how the bundle `rootfs` expects the user namespace to be setup and the runtime SHOULD NOT modify the permissions on the rootfs to realize the mapping.
*hostID* is the starting uid/gid on the host to be mapped to *containerID* which is the starting uid/gid in the container and *size* refers to the number of ids to be mapped.
There is a limit of 5 mappings which is the Linux kernel hard limit.

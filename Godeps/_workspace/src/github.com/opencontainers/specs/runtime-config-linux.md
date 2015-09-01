## Namespaces

A namespace wraps a global system resource in an abstraction that makes it appear to the processes within the namespace that they have their own isolated instance of the global resource.
Changes to the global resource are visible to other processes that are members of the namespace, but are invisible to other processes.
For more information, see [the man page](http://man7.org/linux/man-pages/man7/namespaces.7.html).

Namespaces are specified in the spec as an array of entries.
Each entry has a type field with possible values described below and an optional path element.
If a path is specified, that particular file is used to join that type of namespace.
Also, when a path is specified, a runtime MUST assume that the setup for that particular namespace has already been done and error out if the config specifies anything else related to that namespace.

```json
    "namespaces": [
        {
            "type": "pid",
            "path": "/proc/1234/ns/pid"
        },
        {
            "type": "network",
            "path": "/var/run/netns/neta"
        },
        {
            "type": "mount",
        },
        {
            "type": "ipc",
        },
        {
            "type": "uts",
        },
        {
            "type": "user",
        },
    ]
```

#### Namespace types

* **pid** processes inside the container will only be able to see other processes inside the same container.
* **network** the container will have its own network stack.
* **mount** the container will have an isolated mount table.
* **ipc** processes inside the container will only be able to communicate to other processes inside the same
container via system level IPC.
* **uts** the container will be able to have its own hostname and domain name.
* **user** the container will be able to remap user and group IDs from the host to local users and groups
within the container.

## Devices

Devices is an array specifying the list of devices to be created in the container.
Next parameters can be specified:

* type - type of device: 'c', 'b', 'u' or 'p'. More info in `man mknod`
* path - full path to device inside container
* major, minor - major, minor numbers for device. More info in `man mknod`.
                 There is special value: `-1`, which means `*` for `device`
                 cgroup setup.
* permissions - cgroup permissions for device. A composition of 'r'
                (read), 'w' (write), and 'm' (mknod).
* fileMode - file mode for device file
* uid - uid of device owner
* gid - gid of device owner

```json
   "devices": [
        {
            "path": "/dev/random",
            "type": "c",
            "major": 1,
            "minor": 8,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/urandom",
            "type": "c",
            "major": 1,
            "minor": 9,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/null",
            "type": "c",
            "major": 1,
            "minor": 3,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/zero",
            "type": "c",
            "major": 1,
            "minor": 5,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/tty",
            "type": "c",
            "major": 5,
            "minor": 0,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        },
        {
            "path": "/dev/full",
            "type": "c",
            "major": 1,
            "minor": 7,
            "permissions": "rwm",
            "fileMode": 0666,
            "uid": 0,
            "gid": 0
        }
    ]
```

## Control groups

Also known as cgroups, they are used to restrict resource usage for a container and handle device access.
cgroups provide controls to restrict cpu, memory, IO, pids and network for the container.
For more information, see the [kernel cgroups documentation](https://www.kernel.org/doc/Documentation/cgroups/cgroups.txt).

The path to the cgroups can to be specified in the Spec via `cgroupsPath`.
`cgroupsPath` is expected to be relative to the cgroups mount point.
If not specified, cgroups will be created under '/'.
Implementations of the Spec can choose to name cgroups in any manner.
The Spec does not include naming schema for cgroups.
The Spec does not support [split hierarchy](https://www.kernel.org/doc/Documentation/cgroups/unified-hierarchy.txt).
The cgroups will be created if they don't exist.

```json
   "cgroupsPath": "/myRuntime/myContainer"
```

`cgroupsPath` can be used to either control the cgroups hierarchy for containers or to run a new process in an existing container.

Optionally, cgroups limits can be specified via `resources`.

```json
    "resources": {
        "disableOOMKiller": false,
        "memory": {
            "limit": 0,
            "reservation": 0,
            "swap": 0,
            "kernel": 0,
            "swappiness": -1
        },
        "cpu": {
            "shares": 0,
            "quota": 0,
            "period": 0,
            "realtimeRuntime": 0,
            "realtimePeriod": 0,
            "cpus": "",
            "mems": ""
        },
        "blockIO": {
            "blkioWeight": 0,
            "blkioWeightDevice": "",
            "blkioThrottleReadBpsDevice": "",
            "blkioThrottleWriteBpsDevice": "",
            "blkioThrottleReadIopsDevice": "",
            "blkioThrottleWriteIopsDevice": ""
        },
        "hugepageLimits": null,
        "network": {
            "classId": "",
            "priorities": null
        }
    }
```

Do not specify `resources` unless limits have to be updated.
For example, to run a new process in an existing container without updating limits, `resources` need not be specified.

## Sysctl

sysctl allows kernel parameters to be modified at runtime for the container.
For more information, see [the man page](http://man7.org/linux/man-pages/man8/sysctl.8.html)

```json
   "sysctl": {
        "net.ipv4.ip_forward": "1",
        "net.core.somaxconn": "256"
   }
```

## Rlimits

```json
   "rlimits": [
        {
            "type": "RLIMIT_NPROC",
            "soft": 1024,
            "hard": 102400
        }
   ]
```

rlimits allow setting resource limits.
`type` is a string with a value from those defined in [the man page](http://man7.org/linux/man-pages/man2/setrlimit.2.html).
The kernel enforces the `soft` limit for a resource while the `hard` limit acts as a ceiling for that value that could be set by an unprivileged process.

## SELinux process label

SELinux process label specifies the label with which the processes in a container are run.
For more information about SELinux, see  [Selinux documentation](http://selinuxproject.org/page/Main_Page)
```json
   "selinuxProcessLabel": "system_u:system_r:svirt_lxc_net_t:s0:c124,c675"
```

## Apparmor profile

Apparmor profile specifies the name of the apparmor profile that will be used for the container.
For more information about Apparmor, see [Apparmor documentation](https://wiki.ubuntu.com/AppArmor)

```json
   "apparmorProfile": "acme_secure_profile"
```

## seccomp

Seccomp provides application sandboxing mechanism in the Linux kernel.
Seccomp configuration allows one to configure actions to take for matched syscalls and furthermore also allows matching on values passed as arguments to syscalls.
For more information about Seccomp, see [Seccomp kernel documentation](https://www.kernel.org/doc/Documentation/prctl/seccomp_filter.txt)
The actions and operators are strings that match the definitions in seccomp.h from [libseccomp](https://github.com/seccomp/libseccomp) and are translated to corresponding values.

```json
   "seccomp": {
       "defaultAction": "SCMP_ACT_ALLOW",
       "syscalls": [
           {
               "name": "getcwd",
               "action": "SCMP_ACT_ERRNO"
           }
       ]
   }
```

## Rootfs Mount Propagation

rootfsPropagation sets the rootfs's mount propagation.
Its value is either slave, private, or shared.
[The kernel doc](https://www.kernel.org/doc/Documentation/filesystems/sharedsubtree.txt) has more information about mount propagation.

```json
    "rootfsPropagation": "slave",
```

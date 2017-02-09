# <a name="containerConfigurationFile" />Container Configuration file

The container's top-level directory MUST contain a configuration file called `config.json`.
The canonical schema is defined in this document, but there is a JSON Schema in [`schema/config-schema.json`](schema/config-schema.json) and Go bindings in [`specs-go/config.go`](specs-go/config.go).
[Platform](spec.md#platforms)-specific configuration schema are defined in the [platform-specific documents](#platform-specific-configuration) linked below.
For properties that are only defined for some [platforms](spec.md#platforms), the Go property has a `platform` tag listing those protocols (e.g. `platform:"linux,solaris"`).

The configuration file contains metadata necessary to implement standard operations against the container.
This includes the process to run, environment variables to inject, sandboxing features to use, etc.

Below is a detailed description of each field defined in the configuration format and valid values are specified.
Platform-specific fields are identified as such.
For all platform-specific configuration values, the scope defined below in the [Platform-specific configuration](#platform-specific-configuration) section applies.


## <a name="configSpecificationVersion" />Specification version

* **`ociVersion`** (string, REQUIRED) MUST be in [SemVer v2.0.0][semver-v2.0.0] format and specifies the version of the Open Container Runtime Specification with which the bundle complies.
The Open Container Runtime Specification follows semantic versioning and retains forward and backward compatibility within major versions.
For example, if a configuration is compliant with version 1.1 of this specification, it is compatible with all runtimes that support any 1.1 or later release of this specification, but is not compatible with a runtime that supports 1.0 and not 1.1.

### Example

```json
    "ociVersion": "0.1.0"
```

## <a name="configRoot" />Root

**`root`** (object, REQUIRED) specifies the container's root filesystem.

* **`path`** (string, REQUIRED) Specifies the path to the root filesystem for the container.
  The path is either an absolute path or a relative path to the bundle.
  On Linux, for example, with a bundle at `/to/bundle` and a root filesystem at `/to/bundle/rootfs`, the `path` value can be either `/to/bundle/rootfs` or `rootfs`.
  A directory MUST exist at the path declared by the field.
* **`readonly`** (bool, OPTIONAL) If true then the root filesystem MUST be read-only inside the container, defaults to false.

### Example

```json
"root": {
    "path": "rootfs",
    "readonly": true
}
```

## <a name="configMounts" />Mounts

**`mounts`** (array, OPTIONAL) specifies additional mounts beyond [`root`](#root-configuration).
The runtime MUST mount entries in the listed order.
For Linux, the parameters are as documented in [mount(2)][mount.2] system call man page.
For Solaris, the mount entry corresponds to the 'fs' resource in the [zonecfg(1M)][zonecfg.1m] man page.
For Windows, see [mountvol][mountvol] and [SetVolumeMountPoint][set-volume-mountpoint] for details.


* **`destination`** (string, REQUIRED) Destination of mount point: path inside container.
  This value MUST be an absolute path.
  * Windows: one mount destination MUST NOT be nested within another mount (e.g., c:\\foo and c:\\foo\\bar).
  * Solaris: corresponds to "dir" of the fs resource in [zonecfg(1M)][zonecfg.1m].
* **`type`** (string, OPTIONAL) The filesystem type of the filesystem to be mounted.
  * Linux: valid *filesystemtype* supported by the kernel as listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660").
  * Windows: the type of file system on the volume, e.g. "ntfs".
  * Solaris: corresponds to "type" of the fs resource in [zonecfg(1M)][zonecfg.1m].
* **`source`** (string, OPTIONAL) A device name, but can also be a directory name or a dummy.
  * Windows: the volume name that is the target of the mount point, \\?\Volume\{GUID}\ (on Windows source is called target).
  * Solaris: corresponds to "special" of the fs resource in [zonecfg(1M)][zonecfg.1m].
* **`options`** (list of strings, OPTIONAL) Mount options of the filesystem to be used.
  * Linux: supported options are listed in the [mount(8)][mount.8] man page. Note both [filesystem-independent][mount.8-filesystem-independent] and [filesystem-specific][mount.8-filesystem-specific] options are listed.
  * Solaris: corresponds to "options" of the fs resource in [zonecfg(1M)][zonecfg.1m].

### Example (Linux)

```json
"mounts": [
    {
        "destination": "/tmp",
        "type": "tmpfs",
        "source": "tmpfs",
        "options": ["nosuid","strictatime","mode=755","size=65536k"]
    },
    {
        "destination": "/data",
        "type": "bind",
        "source": "/volumes/testing",
        "options": ["rbind","rw"]
    }
]
```

### Example (Windows)

```json
"mounts": [
    "myfancymountpoint": {
        "destination": "C:\\Users\\crosbymichael\\My Fancy Mount Point\\",
        "type": "ntfs",
        "source": "\\\\?\\Volume\\{2eca078d-5cbc-43d3-aff8-7e8511f60d0e}\\",
        "options": []
    }
]
```

### Example (Solaris)

```json
"mounts": [
    {
        "destination": "/opt/local",
        "type": "lofs",
        "source": "/usr/local",
        "options": ["ro","nodevices"]
    },
    {
        "destination": "/opt/sfw",
        "type": "lofs",
        "source": "/opt/sfw"
    }
]
```

## <a name="configProcess" />Process

**`process`** (object, REQUIRED) specifies the container process.

* **`terminal`** (bool, OPTIONAL) specifies whether a terminal is attached to that process, defaults to false.
  As an example, if set to true on Linux a pseudoterminal pair is allocated for the container process and the pseudoterminal slave is duplicated on the container process's [standard streams][stdin.3].
* **`consoleSize`** (object, OPTIONAL) specifies the console size of the terminal if attached, containing the following properties:
  * **`height`** (uint, REQUIRED)
  * **`width`** (uint, REQUIRED)
* **`cwd`** (string, REQUIRED) is the working directory that will be set for the executable.
  This value MUST be an absolute path.
* **`env`** (array of strings, OPTIONAL) with the same semantics as [IEEE Std 1003.1-2001's `environ`][ieee-1003.1-2001-xbd-c8.1].
* **`args`** (array of strings, REQUIRED) with similar semantics to [IEEE Std 1003.1-2001 `execvp`'s *argv*][ieee-1003.1-2001-xsh-exec].
  This specification extends the IEEE standard in that at least one entry is REQUIRED, and that entry is used with the same semantics as `execvp`'s *file*.
* **`capabilities`** (object, OPTIONAL) is an object containing arrays that specifies the sets of capabilities for the process(es) inside the container. Valid values are platform-specific. For example, valid values for Linux are defined in the [capabilities(7)][capabilities.7] man page.
  capabilities contains the following properties:
    * **`effective`** (array of strings, OPTIONAL) - the `effective` field is an array of effective capabilities that are kept for the process.
    * **`bounding`** (array of strings, OPTIONAL) - the `bounding` field is an array of bounding capabilities that are kept for the process.
    * **`inheritable`** (array of strings, OPTIONAL) - the `inheritable` field is an array of inheritable capabilities that are kept for the process.
    * **`permitted`** (array of strings, OPTIONAL) - the `permitted` field is an array of permitted capabilities that are kept for the process.
    * **`ambient`** (array of strings, OPTIONAL) - the `ambient` field is an array of ambient capabilities that are kept for the process.
* **`rlimits`** (array of objects, OPTIONAL) allows setting resource limits for a process inside the container.
  Each entry has the following structure:

    * **`type`** (string, REQUIRED) - the platform resource being limited, for example on Linux as defined in the [setrlimit(2)][setrlimit.2] man page.
    * **`soft`** (uint64, REQUIRED) - the value of the limit enforced for the corresponding resource.
    * **`hard`** (uint64, REQUIRED) - the ceiling for the soft limit that could be set by an unprivileged process. Only a privileged process (e.g. under Linux: one with the CAP_SYS_RESOURCE capability) can raise a hard limit.

    If `rlimits` contains duplicated entries with same `type`, the runtime MUST error out.

* **`noNewPrivileges`** (bool, OPTIONAL) setting `noNewPrivileges` to true prevents the processes in the container from gaining additional privileges.
  As an example, the ['no_new_privs'][no-new-privs] article in the kernel documentation has information on how this is achieved using a prctl system call on Linux.

For Linux-based systems the process structure supports the following process specific fields.

* **`apparmorProfile`** (string, OPTIONAL) specifies the name of the AppArmor profile to be applied to processes in the container.
  For more information about AppArmor, see [AppArmor documentation][apparmor].
* **`selinuxLabel`** (string, OPTIONAL) specifies the SELinux label to be applied to the processes in the container.
  For more information about SELinux, see  [SELinux documentation][selinux].

### <a name="configUser" />User

The user for the process is a platform-specific structure that allows specific control over which user the process runs as.

#### <a name="configLinuxAndSolarisUser" />Linux and Solaris User

For Linux and Solaris based systems the user structure has the following fields:

* **`uid`** (int, REQUIRED) specifies the user ID in the [container namespace](glossary.md#container-namespace).
* **`gid`** (int, REQUIRED) specifies the group ID in the [container namespace](glossary.md#container-namespace).
* **`additionalGids`** (array of ints, OPTIONAL) specifies additional group IDs (in the [container namespace](glossary.md#container-namespace) to be added to the process.

_Note: symbolic name for uid and gid, such as uname and gname respectively, are left to upper levels to derive (i.e. `/etc/passwd` parsing, NSS, etc)_

### Example (Linux)

```json
"process": {
    "terminal": true,
    "consoleSize": {
        "height": 25,
        "width": 80
    },
    "user": {
        "uid": 1,
        "gid": 1,
        "additionalGids": [5, 6]
    },
    "env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "TERM=xterm"
    ],
    "cwd": "/root",
    "args": [
        "sh"
    ],
    "apparmorProfile": "acme_secure_profile",
    "selinuxLabel": "system_u:system_r:svirt_lxc_net_t:s0:c124,c675",
    "noNewPrivileges": true,
    "capabilities": {
        "bounding": [
            "CAP_AUDIT_WRITE",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE"
        ],
       "permitted": [
            "CAP_AUDIT_WRITE",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE"
        ],
       "inheritable": [
            "CAP_AUDIT_WRITE",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE"
        ],
        "effective": [
            "CAP_AUDIT_WRITE",
            "CAP_KILL",
        ],
        "ambient": [
            "CAP_NET_BIND_SERVICE"
        ]
    },
    "rlimits": [
        {
            "type": "RLIMIT_NOFILE",
            "hard": 1024,
            "soft": 1024
        }
    ]
}
```
### Example (Solaris)

```json
"process": {
    "terminal": true,
    "consoleSize": {
        "height": 25,
        "width": 80
    },
    "user": {
        "uid": 1,
        "gid": 1,
        "additionalGids": [2, 8]
    },
    "env": [
        "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
        "TERM=xterm"
    ],
    "cwd": "/root",
    "args": [
        "/usr/bin/bash"
    ]
}
```

#### <a name="configWindowsUser" />Windows User

For Windows based systems the user structure has the following fields:

* **`username`** (string, OPTIONAL) specifies the user name for the process.

### Example (Windows)

```json
"process": {
    "terminal": true,
    "user": {
        "username": "containeradministrator"
    },
    "env": [
        "VARIABLE=1"
    ],
    "cwd": "c:\\foo",
    "args": [
        "someapp.exe",
    ]
}
```


## <a name="configHostname" />Hostname

* **`hostname`** (string, OPTIONAL) specifies the container's hostname as seen by processes running inside the container.
  On Linux, for example, this will change the hostname in the [container](glossary.md#container-namespace) [UTS namespace][uts-namespace.7].
  Depending on your [namespace configuration](config-linux.md#namespaces), the container UTS namespace may be the [runtime UTS namespace](glossary.md#runtime-namespace).

### Example

```json
"hostname": "mrsdalloway"
```

## <a name="configPlatform" />Platform

**`platform`** (object, REQUIRED) specifies the configuration's target platform.

* **`os`** (string, REQUIRED) specifies the operating system family of the container configuration's specified [`root`](#root-configuration) file system bundle.
  The runtime MUST generate an error if it does not support the specified **`os`**.
  Bundles SHOULD use, and runtimes SHOULD understand, **`os`** entries listed in the Go Language document for [`$GOOS`][go-environment].
  If an operating system is not included in the `$GOOS` documentation, it SHOULD be submitted to this specification for standardization.
* **`arch`** (string, REQUIRED) specifies the instruction set for which the binaries in the specified [`root`](#root-configuration) file system bundle have been compiled.
  The runtime MUST generate an error if it does not support the specified **`arch`**.
  Values for **`arch`** SHOULD use, and runtimes SHOULD understand, **`arch`** entries listed in the Go Language document for [`$GOARCH`][go-environment].
  If an architecture is not included in the `$GOARCH` documentation, it SHOULD be submitted to this specification for standardization.

### Example

```json
"platform": {
    "os": "linux",
    "arch": "amd64"
}
```

## <a name="configPlatformSpecificConfiguration" />Platform-specific configuration

[**`platform.os`**](#platform) is used to specify platform-specific configuration.
Runtime implementations MAY support any valid values for platform-specific fields as part of this configuration.
Implementations MUST error out when invalid values are encountered and MUST generate an error message and error out when encountering valid values it chooses to not support.

* **`linux`** (object, OPTIONAL) [Linux-specific configuration](config-linux.md).
  This MAY be set if **`platform.os`** is `linux` and MUST NOT be set otherwise.
* **`windows`** (object, OPTIONAL) [Windows-specific configuration](config-windows.md).
  This MAY be set if **`platform.os`** is `windows` and MUST NOT be set otherwise.
* **`solaris`** (object, OPTIONAL) [Solaris-specific configuration](config-solaris.md).
  This MAY be set if **`platform.os`** is `solaris` and MUST NOT be set otherwise.

### Example (Linux)

```json
{
    "platform": {
        "os": "linux",
        "arch": "amd64"
    },
    "linux": {
        "namespaces": [
          {
            "type": "pid"
          }
        ]
    }
}
```

## <a name="configHooks" />Hooks

Hooks allow for the configuration of custom actions related to the [lifecycle](runtime.md#lifecycle) of the container.

* **`hooks`** (object, OPTIONAL) MAY contain any of the following properties:
  * **`prestart`** (array, OPTIONAL) is an array of [pre-start hooks](#prestart).
    Entries in the array contain the following properties:
    * **`path`** (string, REQUIRED) with similar semantics to [IEEE Std 1003.1-2001 `execv`'s *path*][ieee-1003.1-2001-xsh-exec].
      This specification extends the IEEE standard in that **`path`** MUST be absolute.
    * **`args`** (array of strings, OPTIONAL) with the same semantics as [IEEE Std 1003.1-2001 `execv`'s *argv*][ieee-1003.1-2001-xsh-exec].
    * **`env`** (array of strings, OPTIONAL) with the same semantics as [IEEE Std 1003.1-2001's `environ`][ieee-1003.1-2001-xbd-c8.1].
    * **`timeout`** (int, OPTIONAL) is the number of seconds before aborting the hook.
  * **`poststart`** (array, OPTIONAL) is an array of [post-start hooks](#poststart).
    Entries in the array have the same schema as pre-start entries.
  * **`poststop`** (array, OPTIONAL) is an array of [post-stop hooks](#poststop).
    Entries in the array have the same schema as pre-start entries.

Hooks allow users to specify programs to run before or after various lifecycle events.
Hooks MUST be called in the listed order.
The [state](runtime.md#state) of the container MUST be passed to hooks over stdin so that they may do work appropriate to the current state of the container.

### <a name="configHooksPrestart" />Prestart

The pre-start hooks MUST be called after the [`start`](runtime.md#start) operation is called but [before the user-specified program command is executed](runtime.md#lifecycle).
On Linux, for example, they are called after the container namespaces are created, so they provide an opportunity to customize the container (e.g. the network namespace could be specified in this hook).

### <a name="configHooksPoststart" />Poststart

The post-start hooks MUST be called [after the user-specified process is executed](runtime#lifecycle) but before the [`start`](runtime.md#start) operation returns.
For example, this hook can notify the user that the container process is spawned.

### <a name="configHooksPoststop" />Poststop

The post-stop hooks MUST be called [after the container is deleted](runtime#lifecycle) but before the [`delete`](runtime.md#delete) operation returns.
Cleanup or debugging functions are examples of such a hook.

### Example

```json
    "hooks": {
        "prestart": [
            {
                "path": "/usr/bin/fix-mounts",
                "args": ["fix-mounts", "arg1", "arg2"],
                "env":  [ "key1=value1"]
            },
            {
                "path": "/usr/bin/setup-network"
            }
        ],
        "poststart": [
            {
                "path": "/usr/bin/notify-start",
                "timeout": 5
            }
        ],
        "poststop": [
            {
                "path": "/usr/sbin/cleanup.sh",
                "args": ["cleanup.sh", "-f"]
            }
        ]
    }
```

## <a name="configAnnotations" />Annotations

**`annotations`** (object, OPTIONAL) contains arbitrary metadata for the container.
This information MAY be structured or unstructured.
Annotations MUST be a key-value map.
If there are no annotations then this property MAY either be absent or an empty map.

Keys MUST be strings.
Keys MUST be unique within this map.
Keys MUST NOT be an empty string.
Keys SHOULD be named using a reverse domain notation - e.g. `com.example.myKey`.
Keys using the `org.opencontainers` namespace are reserved and MUST NOT be used by subsequent specifications.
Implementations that are reading/processing this configuration file MUST NOT generate an error if they encounter an unknown annotation key.

Values MUST be strings.
Values MAY be an empty string.

```json
"annotations": {
    "com.example.gpu-cores": "2"
}
```

## <a name="configExtensibility" />Extensibility
Implementations that are reading/processing this configuration file MUST NOT generate an error if they encounter an unknown property.
Instead they MUST ignore unknown properties.

## Configuration Schema Example

Here is a full example `config.json` for reference.

```json
{
    "ociVersion": "0.5.0-dev",
    "platform": {
        "os": "linux",
        "arch": "amd64"
    },
    "process": {
        "terminal": true,
        "user": {
            "uid": 1,
            "gid": 1,
            "additionalGids": [
                5,
                6
            ]
        },
        "args": [
            "sh"
        ],
        "env": [
            "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
            "TERM=xterm"
        ],
        "cwd": "/",
        "capabilities": {
            "bounding": [
                "CAP_AUDIT_WRITE",
                "CAP_KILL",
                "CAP_NET_BIND_SERVICE"
            ],
            "permitted": [
                "CAP_AUDIT_WRITE",
                "CAP_KILL",
                "CAP_NET_BIND_SERVICE"
            ],
            "inheritable": [
                "CAP_AUDIT_WRITE",
                "CAP_KILL",
                "CAP_NET_BIND_SERVICE"
            ],
            "effective": [
                "CAP_AUDIT_WRITE",
                "CAP_KILL",
            ],
            "ambient": [
                "CAP_NET_BIND_SERVICE"
            ]
        },
        "rlimits": [
            {
                "type": "RLIMIT_CORE",
                "hard": 1024,
                "soft": 1024
            },
            {
                "type": "RLIMIT_NOFILE",
                "hard": 1024,
                "soft": 1024
            }
        ],
        "apparmorProfile": "acme_secure_profile",
        "selinuxLabel": "system_u:system_r:svirt_lxc_net_t:s0:c124,c675",
        "noNewPrivileges": true
    },
    "root": {
        "path": "rootfs",
        "readonly": true
    },
    "hostname": "slartibartfast",
    "mounts": [
        {
            "destination": "/proc",
            "type": "proc",
            "source": "proc"
        },
        {
            "destination": "/dev",
            "type": "tmpfs",
            "source": "tmpfs",
            "options": [
                "nosuid",
                "strictatime",
                "mode=755",
                "size=65536k"
            ]
        },
        {
            "destination": "/dev/pts",
            "type": "devpts",
            "source": "devpts",
            "options": [
                "nosuid",
                "noexec",
                "newinstance",
                "ptmxmode=0666",
                "mode=0620",
                "gid=5"
            ]
        },
        {
            "destination": "/dev/shm",
            "type": "tmpfs",
            "source": "shm",
            "options": [
                "nosuid",
                "noexec",
                "nodev",
                "mode=1777",
                "size=65536k"
            ]
        },
        {
            "destination": "/dev/mqueue",
            "type": "mqueue",
            "source": "mqueue",
            "options": [
                "nosuid",
                "noexec",
                "nodev"
            ]
        },
        {
            "destination": "/sys",
            "type": "sysfs",
            "source": "sysfs",
            "options": [
                "nosuid",
                "noexec",
                "nodev"
            ]
        },
        {
            "destination": "/sys/fs/cgroup",
            "type": "cgroup",
            "source": "cgroup",
            "options": [
                "nosuid",
                "noexec",
                "nodev",
                "relatime",
                "ro"
            ]
        }
    ],
    "hooks": {
        "prestart": [
            {
                "path": "/usr/bin/fix-mounts",
                "args": [
                    "fix-mounts",
                    "arg1",
                    "arg2"
                ],
                "env": [
                    "key1=value1"
                ]
            },
            {
                "path": "/usr/bin/setup-network"
            }
        ],
        "poststart": [
            {
                "path": "/usr/bin/notify-start",
                "timeout": 5
            }
        ],
        "poststop": [
            {
                "path": "/usr/sbin/cleanup.sh",
                "args": [
                    "cleanup.sh",
                    "-f"
                ]
            }
        ]
    },
    "linux": {
        "devices": [
            {
                "path": "/dev/fuse",
                "type": "c",
                "major": 10,
                "minor": 229,
                "fileMode": 438,
                "uid": 0,
                "gid": 0
            },
            {
                "path": "/dev/sda",
                "type": "b",
                "major": 8,
                "minor": 0,
                "fileMode": 432,
                "uid": 0,
                "gid": 0
            }
        ],
        "uidMappings": [
            {
                "hostID": 1000,
                "containerID": 0,
                "size": 32000
            }
        ],
        "gidMappings": [
            {
                "hostID": 1000,
                "containerID": 0,
                "size": 32000
            }
        ],
        "sysctl": {
            "net.ipv4.ip_forward": "1",
            "net.core.somaxconn": "256"
        },
        "cgroupsPath": "/myRuntime/myContainer",
        "resources": {
            "network": {
                "classID": 1048577,
                "priorities": [
                    {
                        "name": "eth0",
                        "priority": 500
                    },
                    {
                        "name": "eth1",
                        "priority": 1000
                    }
                ]
            },
            "pids": {
                "limit": 32771
            },
            "hugepageLimits": [
                {
                    "pageSize": "2MB",
                    "limit": 9223372036854772000
                }
            ],
            "oomScoreAdj": 100,
            "memory": {
                "limit": 536870912,
                "reservation": 536870912,
                "swap": 536870912,
                "kernel": 0,
                "kernelTCP": 0,
                "swappiness": 0
            },
            "cpu": {
                "shares": 1024,
                "quota": 1000000,
                "period": 500000,
                "realtimeRuntime": 950000,
                "realtimePeriod": 1000000,
                "cpus": "2-3",
                "mems": "0-7"
            },
            "disableOOMKiller": false,
            "devices": [
                {
                    "allow": false,
                    "access": "rwm"
                },
                {
                    "allow": true,
                    "type": "c",
                    "major": 10,
                    "minor": 229,
                    "access": "rw"
                },
                {
                    "allow": true,
                    "type": "b",
                    "major": 8,
                    "minor": 0,
                    "access": "r"
                }
            ],
            "blockIO": {
                "blkioWeight": 10,
                "blkioLeafWeight": 10,
                "blkioWeightDevice": [
                    {
                        "major": 8,
                        "minor": 0,
                        "weight": 500,
                        "leafWeight": 300
                    },
                    {
                        "major": 8,
                        "minor": 16,
                        "weight": 500
                    }
                ],
                "blkioThrottleReadBpsDevice": [
                    {
                        "major": 8,
                        "minor": 0,
                        "rate": 600
                    }
                ],
                "blkioThrottleWriteIOPSDevice": [
                    {
                        "major": 8,
                        "minor": 16,
                        "rate": 300
                    }
                ]
            }
        },
        "rootfsPropagation": "slave",
        "seccomp": {
            "defaultAction": "SCMP_ACT_ALLOW",
            "architectures": [
                "SCMP_ARCH_X86",
                "SCMP_ARCH_X32"
            ],
            "syscalls": [
                {
                    "names": [
                        "getcwd",
                        "chmod"
                    ],
                    "action": "SCMP_ACT_ERRNO",
                    "comment": "stop exploit x"
                }
            ]
        },
        "namespaces": [
            {
                "type": "pid"
            },
            {
                "type": "network"
            },
            {
                "type": "ipc"
            },
            {
                "type": "uts"
            },
            {
                "type": "mount"
            },
            {
                "type": "user"
            },
            {
                "type": "cgroup"
            }
        ],
        "maskedPaths": [
            "/proc/kcore",
            "/proc/latency_stats",
            "/proc/timer_stats",
            "/proc/sched_debug"
        ],
        "readonlyPaths": [
            "/proc/asound",
            "/proc/bus",
            "/proc/fs",
            "/proc/irq",
            "/proc/sys",
            "/proc/sysrq-trigger"
        ],
        "mountLabel": "system_u:object_r:svirt_sandbox_file_t:s0:c715,c811"
    },
    "annotations": {
        "com.example.key1": "value1",
        "com.example.key2": "value2"
    }
}
```


[apparmor]: https://wiki.ubuntu.com/AppArmor
[selinux]:http://selinuxproject.org/page/Main_Page
[no-new-privs]: https://www.kernel.org/doc/Documentation/prctl/no_new_privs.txt
[semver-v2.0.0]: http://semver.org/spec/v2.0.0.html
[go-environment]: https://golang.org/doc/install/source#environment
[ieee-1003.1-2001-xbd-c8.1]: http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html#tag_08_01
[ieee-1003.1-2001-xsh-exec]: http://pubs.opengroup.org/onlinepubs/009695399/functions/exec.html
[mountvol]: http://ss64.com/nt/mountvol.html
[set-volume-mountpoint]: https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx

[capabilities.7]: http://man7.org/linux/man-pages/man7/capabilities.7.html
[mount.2]: http://man7.org/linux/man-pages/man2/mount.2.html
[mount.8]: http://man7.org/linux/man-pages/man8/mount.8.html
[mount.8-filesystem-independent]: http://man7.org/linux/man-pages/man8/mount.8.html#FILESYSTEM-INDEPENDENT_MOUNT%20OPTIONS
[mount.8-filesystem-specific]: http://man7.org/linux/man-pages/man8/mount.8.html#FILESYSTEM-SPECIFIC_MOUNT%20OPTIONS
[setrlimit.2]: http://man7.org/linux/man-pages/man2/setrlimit.2.html
[stdin.3]: http://man7.org/linux/man-pages/man3/stdin.3.html
[uts-namespace.7]: http://man7.org/linux/man-pages/man7/namespaces.7.html
[zonecfg.1m]: http://docs.oracle.com/cd/E53394_01/html/E54764/zonecfg-1m.html

# Container Configuration file

The container's top-level directory MUST contain a configuration file called `config.json`.
The canonical schema is defined in this document, but there is a JSON Schema in [`schema/config-schema.json`](schema/config-schema.json) and Go bindings in [`specs-go/config.go`](specs-go/config.go).

The configuration file contains metadata necessary to implement standard operations against the container.
This includes the process to run, environment variables to inject, sandboxing features to use, etc.

Below is a detailed description of each field defined in the configuration format.

## Specification version

* **`ociVersion`** (string, REQUIRED) MUST be in [SemVer v2.0.0](http://semver.org/spec/v2.0.0.html) format and specifies the version of the Open Container Runtime Specification with which the bundle complies.
The Open Container Runtime Specification follows semantic versioning and retains forward and backward compatibility within major versions.
For example, if a configuration is compliant with version 1.1 of this specification, it is compatible with all runtimes that support any 1.1 or later release of this specification, but is not compatible with a runtime that supports 1.0 and not 1.1.

### Example

```json
    "ociVersion": "0.1.0"
```

## Root Configuration

**`root`** (object, REQUIRED) configures the container's root filesystem.

* **`path`** (string, REQUIRED) Specifies the path to the root filesystem for the container.
  The path can be an absolute path (starting with /) or a relative path (not starting with /), which is relative to the bundle.
  For example (Linux), with a bundle at `/to/bundle` and a root filesystem at `/to/bundle/rootfs`, the `path` value can be either `/to/bundle/rootfs` or `rootfs`.
  A directory MUST exist at the path declared by the field.
* **`readonly`** (bool, OPTIONAL) If true then the root filesystem MUST be read-only inside the container, defaults to false.

### Example

```json
"root": {
    "path": "rootfs",
    "readonly": true
}
```

## Mounts

**`mounts`** (array, OPTIONAL) configures additional mounts (on top of [`root`](#root-configuration)).
The runtime MUST mount entries in the listed order.
The parameters are similar to the ones in [the Linux mount system call](http://man7.org/linux/man-pages/man2/mount.2.html).

* **`destination`** (string, REQUIRED) Destination of mount point: path inside container.
  For the Windows operating system, one mount destination MUST NOT be nested within another mount (e.g., c:\\foo and c:\\foo\\bar).
* **`type`** (string, REQUIRED) The filesystem type of the filesystem to be mounted.
  Linux: *filesystemtype* argument supported by the kernel are listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660").
  Windows: ntfs.
* **`source`** (string, REQUIRED) A device name, but can also be a directory name or a dummy.
  Windows: the volume name that is the target of the mount point, \\?\Volume\{GUID}\ (on Windows source is called target).
* **`options`** (list of strings, OPTIONAL) Mount options of the filesystem to be used.
  Linux: [supported][mount.8-filesystem-independent] [options][mount.8-filesystem-specific] are listed in [mount(8)][mount.8].

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

See links for details about [mountvol](http://ss64.com/nt/mountvol.html) and [SetVolumeMountPoint](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx) in Windows.


## Process configuration

**`process`** (object, REQUIRED) configures the container process.

* **`terminal`** (bool, OPTIONAL) specifies whether you want a terminal attached to that process, defaults to false.
* **`consoleSize`** (object, OPTIONAL) specifies the console size of the terminal if attached, containing the following properties:
  * **`height`** (uint, REQUIRED)
  * **`width`** (uint, REQUIRED)
* **`cwd`** (string, REQUIRED) is the working directory that will be set for the executable.
  This value MUST be an absolute path.
* **`env`** (array of strings, OPTIONAL) contains a list of variables that will be set in the process's environment prior to execution.
  Elements in the array are specified as Strings in the form "KEY=value".
  The left hand side MUST consist solely of letters, digits, and underscores `_` as outlined in [IEEE Std 1003.1-2001](http://pubs.opengroup.org/onlinepubs/009695399/basedefs/xbd_chap08.html).
* **`args`** (array of strings, REQUIRED) executable to launch and any flags as an array.
  The executable is the first element and MUST be available at the given path inside of the rootfs.
  If the executable path is not an absolute path then the search $PATH is interpreted to find the executable.

For Linux-based systems the process structure supports the following process specific fields:

* **`capabilities`** (array of strings, OPTIONAL) capabilities is an array that specifies Linux capabilities that can be provided to the process inside the container.
Valid values are the strings for capabilities defined in [the man page](http://man7.org/linux/man-pages/man7/capabilities.7.html)
* **`rlimits`** (array of rlimits, OPTIONAL) rlimits is an array of rlimits that allows setting resource limits for a process inside the container.
The kernel enforces the `soft` limit for a resource while the `hard` limit acts as a ceiling for that value that could be set by an unprivileged process.
Valid values for the 'type' field are the resources defined in [the man page](http://man7.org/linux/man-pages/man2/setrlimit.2.html).
* **`apparmorProfile`** (string, OPTIONAL) apparmor profile specifies the name of the apparmor profile that will be used for the container.
For more information about Apparmor, see [Apparmor documentation](https://wiki.ubuntu.com/AppArmor)
* **`selinuxLabel`** (string, OPTIONAL) SELinux process label specifies the label with which the processes in a container are run.
For more information about SELinux, see  [Selinux documentation](http://selinuxproject.org/page/Main_Page)
* **`noNewPrivileges`** (bool, OPTIONAL) setting `noNewPrivileges` to true prevents the processes in the container from gaining additional privileges.
[The kernel doc](https://www.kernel.org/doc/Documentation/prctl/no_new_privs.txt) has more information on how this is achieved using a prctl system call.

### User

The user for the process is a platform-specific structure that allows specific control over which user the process runs as.

#### Linux and Solaris User

For Linux and Solaris based systems the user structure has the following fields:

* **`uid`** (int, REQUIRED) specifies the user ID in the [container namespace][container-namespace].
* **`gid`** (int, REQUIRED) specifies the group ID in the [container namespace][container-namespace].
* **`additionalGids`** (array of ints, OPTIONAL) specifies additional group IDs (in the [container namespace][container-namespace]) to be added to the process.

_Note: symbolic name for uid and gid, such as uname and gname respectively, are left to upper levels to derive (i.e. `/etc/passwd` parsing, NSS, etc)_

_Note: For Solaris, uid and gid specify the uid and gid of the process inside the container and need not be same as in the host._

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
    "capabilities": [
        "CAP_AUDIT_WRITE",
        "CAP_KILL",
        "CAP_NET_BIND_SERVICE"
    ],
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

#### Windows User

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


## Hostname

* **`hostname`** (string, OPTIONAL) configures the container's hostname as seen by processes running inside the container.
  On Linux, you can only set this if your bundle creates a new [UTS namespace][uts-namespace].

### Example

```json
"hostname": "mrsdalloway"
```

## Platform

**`platform`** specifies the configuration's target platform.

* **`os`** (string, REQUIRED) specifies the operating system family this image targets.
  The runtime MUST generate an error if it does not support the configured **`os`**.
  Bundles SHOULD use, and runtimes SHOULD understand, **`os`** entries listed in the Go Language document for [`$GOOS`][go-environment].
  If an operating system is not included in the `$GOOS` documentation, it SHOULD be submitted to this specification for standardization.
* **`arch`** (string, REQUIRED) specifies the instruction set for which the binaries in the image have been compiled.
  The runtime MUST generate an error if it does not support the configured **`arch`**.
  Values for **`arch`** SHOULD use, and runtimes SHOULD understand, **`arch`** entries listed in the Go Language document for [`$GOARCH`][go-environment].
  If an architecture is not included in the `$GOARCH` documentation, it SHOULD be submitted to this specification for standardization.

### Example

```json
"platform": {
    "os": "linux",
    "arch": "amd64"
}
```

## Platform-specific configuration

[**`platform.os`**](#platform) is used to lookup further platform-specific configuration.

* **`linux`** (object, OPTIONAL) [Linux-specific configuration](config-linux.md).
  This SHOULD only be set if **`platform.os`** is `linux`.
* **`solaris`** (object, OPTIONAL) [Solaris-specific configuration](config-solaris.md).
  This SHOULD only be set if **`platform.os`** is `solaris`.
* **`windows`** (object, optional) [Windows-specific configuration](config-windows.md).
  This SHOULD only be set if **`platform.os`** is `windows`.

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

## Hooks

**`hooks`** (object, OPTIONAL) configures callbacks for container lifecycle events.
Lifecycle hooks allow custom events for different points in a container's runtime.
Presently there are `Prestart`, `Poststart` and `Poststop`.

* [`Prestart`](#prestart) is a list of hooks to be run before the container process is executed
* [`Poststart`](#poststart) is a list of hooks to be run immediately after the container process is started
* [`Poststop`](#poststop) is a list of hooks to be run after the container process exits

Hooks allow one to run code before/after various lifecycle events of the container.
Hooks MUST be called in the listed order.
The state of the container is passed to the hooks over stdin, so the hooks could get the information they need to do their work.

Hook paths are absolute and are executed from the host's filesystem in the [runtime namespace][runtime-namespace].

### Prestart

The pre-start hooks are called after the container process is spawned, but before the user supplied command is executed.
They are called after the container namespaces are created on Linux, so they provide an opportunity to customize the container.
In Linux, for e.g., the network namespace could be configured in this hook.

If a hook returns a non-zero exit code, then an error including the exit code and the stderr is returned to the caller and the container is torn down.

### Poststart

The post-start hooks are called after the user process is started.
For example this hook can notify user that real process is spawned.

If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

### Poststop

The post-stop hooks are called after the container process is stopped.
Cleanup or debugging could be performed in such a hook.
If a hook returns a non-zero exit code, then an error is logged and the remaining hooks are executed.

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

`path` is REQUIRED for a hook.
`args` and `env` are OPTIONAL.
`timeout` is the number of seconds before aborting the hook.
The semantics are the same as `Path`, `Args` and `Env` in [golang Cmd](https://golang.org/pkg/os/exec/#Cmd).

## Annotations

**`annotations`** (object, OPTIONAL) contains arbitrary metadata for the container.
This information MAY be structured or unstructured.
Annotations MUST be a key-value map where both the key and value MUST be strings.
While the value MUST be present, it MAY be an empty string.
Keys MUST be unique within this map, and best practice is to namespace the keys.
Keys SHOULD be named using a reverse domain notation - e.g. `com.example.myKey`.
Keys using the `org.opencontainers` namespace are reserved and MUST NOT be used by subsequent specifications.
If there are no annotations then this property MAY either be absent or an empty map.
Implementations that are reading/processing this configuration file MUST NOT generate an error if they encounter an unknown annotation key.

```json
"annotations": {
    "com.example.gpu-cores": "2"
}
```

## Extensibility
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
        "capabilities": [
            "CAP_AUDIT_WRITE",
            "CAP_KILL",
            "CAP_NET_BIND_SERVICE"
        ],
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
                "SCMP_ARCH_X86"
            ],
            "syscalls": [
                {
                    "name": "getcwd",
                    "action": "SCMP_ACT_ERRNO"
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

[container-namespace]: glossary.md#container-namespace
[go-environment]: https://golang.org/doc/install/source#environment
[runtime-namespace]: glossary.md#runtime-namespace
[uts-namespace]: http://man7.org/linux/man-pages/man7/namespaces.7.html
[mount.8-filesystem-independent]: http://man7.org/linux/man-pages/man8/mount.8.html#FILESYSTEM-INDEPENDENT_MOUNT%20OPTIONS
[mount.8-filesystem-specific]: http://man7.org/linux/man-pages/man8/mount.8.html#FILESYSTEM-SPECIFIC_MOUNT%20OPTIONS
[mount.8]: http://man7.org/linux/man-pages/man8/mount.8.html

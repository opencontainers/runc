# Configuration file

The container’s top-level directory MUST contain a configuration file called config.json. The configuration file MUST comply with the Open Container Configuration JSON Schema attached to this document. For now the schema is defined in [spec.go](https://github.com/opencontainers/runc/blob/master/spec.go) and [spec_linux.go](https://github.com/opencontainers/runc/blob/master/spec_linux.go), this will be moved to a JSON schema overtime.

The configuration file contains metadata necessary to implement standard operations against the container. This includes processes to run, environment variables to inject, sandboxing features to use, etc.

Below is a detailed description of each field defined in the configuration format.

## Manifest version

The `version` element specifies the version of the OCF specification which the container complies with. If the container is compliant with multiple versions, it SHOULD advertise the most recent known version to be supported.

*Linux example*

```
    "version": "1",
```

## File system configuration

Each container has exactly one *root filesystem*, and any number of optional *mounted filesystems*. Both need to be declared in the manifest.

The rootfs string element specifies the path to the root file system for the container, relative to the path where the manifest is. A directory MUST exist at the relative path declared by the field.

The readonlyRootfs is an optional boolean element which defaults to false. If it is true, access to the root file system MUST be read-only for all processes running inside it.  whether you want the root file system to be readonly or not for the processes running on it.

*Example (Linux)*

```
    "rootfs": "rootfs",
    "readonlyRootfs": true,
```

*Example (Windows)*

```
    "rootfs": "My Fancy Root FS",
    "readonlyRootfs": true,
```

Additional file systems can be declared as "mounts", declared by the the array element mounts. The parameters are similar to the ones in Linux mount system call. [http://linux.die.net/man/2/mount](http://linux.die.net/man/2/mount)

type: Linux, *filesystemtype* argument supported by the kernel are listed in */proc/filesystems* (e.g., "minix", "ext2", "ext3", "jfs", "xfs", "reiserfs", "msdos", "proc", "nfs", "iso9660"). Windows: ntfs

source: a device name, but can also be a directory name or a dummy. Windows, the volume name that is the target of the mount point. \\?\Volume\{GUID}\ (on Windows source is called target)

destination: where the file system is mounted in the container.

options: in the fstab format [https://wiki.archlinux.org/index.php/Fstab](https://wiki.archlinux.org/index.php/Fstab).

*Example (Linux)*

```
    "mounts": [
        {
            "type": "proc",
            "source": "proc",
            "destination": "/proc",
            "options": ""
        },
        {
            "type": "tmpfs",
            "source": "tmpfs",
            "destination": "/dev",
            "options": "nosuid,strictatime,mode=755,size=65536k"
        },
        {
            "type": "devpts",
            "source": "devpts",
            "destination": "/dev/pts",
            "options": "nosuid,noexec,newinstance,ptmxmode=0666,mode=0620,gid=5"
        },
        {
            "type": "tmpfs",
            "source": "shm",
            "destination": "/dev/shm",
            "options": "nosuid,noexec,nodev,mode=1777,size=65536k"
        },
    ]
```

*Example (Windows)*
```
    "mounts": [
        {
            "type": "ntfs",
            "source": "\\?\Volume\{2eca078d-5cbc-43d3-aff8-7e8511f60d0e}\

",
            "destination": "C:\Users\crosbymichael\My Fancy Mount Point\",
            "options": ""
        }
```

See links for details about mountvol in Windows.

[https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx](https://msdn.microsoft.com/en-us/library/windows/desktop/aa365561(v=vs.85).aspx)

[http://ss64.com/nt/mountvol.html](http://ss64.com/nt/mountvol.html)

### Processes configuration

- Command-line arguments
- Terminal allocation
- User ID
- Environment variables
- Working directory

*Example (Linux)*
```
    "processes": [
        {
            "tty": true,
            "user": "daemon",
            "args": [
                "sh"
            ],
            "env": [
                "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
                "TERM=xterm"
            ],
            "cwd": ""
        }
    ],
```

The processes to be created inside the container are specified in a processes array. They are started in order.

```
    "processes": [...]
```

The command to start a process is specified in an array of args. It will be run in the working directory specified in the string cwd.

Environment variables are specified is an array called env.

Elements in the array are specified as Strings in the form "KEY=value"

The user inside the container under which the process is running is specified under the user key.

tty is a boolean that lets you specify whether you want a terminal attached to that process. tty cannot be set to true for more than one process in the array, else oc returns the error code THERE_CAN_BE_ONLY_ONE_TTY.

*Example (Windows)*

```
    "processes": [
        {
            "tty": true,
            "user": "Contoso\ScottGu",
            "args": [
                "cmd.exe"
            ],
            "env": [
                "PATH=D:\Windows\Microsoft.NET\Framework\v4.0.30319;D:\Program Files (x86)\Git\bin",
                "TERM=cygwin"
            ],
            "cwd": ""
        }
    ],
```

hostname is a string specifying the hostname for that container as it is accessible to processes running in it.

### Resource Constraints

*Example*

```
    "hostname": "mrsdalloway",
```

The number of CPUs is specified as a positive decimal under the key cpus.

The amount of memory allocated to this container is specified under the memory key, as an integer and is expressed in MBb.

If the cpu or memory requested are too high for the underlying environment capabilities, an error code NOT_ENOUGH_CPU or NOT_ENOUGH_MEM will be returned.


### Access to devices

```
   "devices": [
        "null",
        "random",
        "full",
        "tty",
        "zero",
        "urandom"
    ],
```

Devices is an array specifying the list of devices from the host to make available in the container.

The array contains names: for each name, the device /dev/<name> will be made available inside the container.

## Machine-specific configuration

```
    "os": "linux",
    "arch": "amd64",
```

os specifies the operating system family this image must run on.

arch specifies the instruction set for which the binaries in the image have been compiled.

values for os and arch must be in the list specified by by the Go Language documentation for $GOOS and $GOARCH https://golang.org/doc/install/source#environment

OS or architecture specific settings can be added in the json file. They will be interpreted by the implementation depending on the os and arch values specified at the top of the manifest.

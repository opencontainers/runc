OCF spec draft

[[TOC]]

# The 5 principles of Standard Containers

Docker defines a unit of software delivery called a Standard Container. The goal of a Standard Container is to encapsulate a software component and all its dependencies in a format that is self-describing and portable, so that any compliant runtime can run it without extra dependencies, regardless of the underlying machine and the contents of the container.

The specification for Standard Containers is straightforward. It mostly defines 1) a file format, 2) a set of standard operations, and 3) an execution environment.

A great analogy for this is the shipping container. Just like how Standard Containers are a fundamental unit of software delivery, shipping containers are a fundamental unit of physical delivery.

## 1. Standard operations

Just like shipping containers, Standard Containers define a set of STANDARD OPERATIONS. Shipping containers can be lifted, stacked, locked, loaded, unloaded and labelled. Similarly, Standard Containers can be started, stopped, copied, snapshotted, downloaded, uploaded and tagged.

## 2. Content-agnostic

Just like shipping containers, Standard Containers are CONTENT-AGNOSTIC: all standard operations have the same effect regardless of the contents. A shipping container will be stacked in exactly the same way whether it contains Vietnamese powder coffee or spare Maserati parts. Similarly, Standard Containers are started or uploaded in the same way whether they contain a postgres database, a php application with its dependencies and application server, or Java build artifacts.

## 3. Infrastructure-agnostic

Both types of containers are INFRASTRUCTURE-AGNOSTIC: they can be transported to thousands of facilities around the world, and manipulated by a wide variety of equipment. A shipping container can be packed in a factory in Ukraine, transported by truck to the nearest

routing center, stacked onto a train, loaded into a German boat by an Australian-built crane, stored in a warehouse at a US facility, etc. Similarly, a standard container can be bundled on my laptop, uploaded to S3, downloaded, run and snapshotted by a build server at Equinix in Virginia, uploaded to 10 staging servers in a home-made Openstack cluster, then sent to 30 production instances across 3 EC2 regions.

## 4. Designed for automation

Because they offer the same standard operations regardless of content and infrastructure, Standard Containers, just like their physical counterparts, are extremely well-suited for automation. In fact, you could say automation is their secret weapon.

Many things that once required time-consuming and error-prone human effort can now be programmed. Before shipping containers, a bag of powder coffee was hauled, dragged, dropped, rolled and stacked by 10 different people in 10 different locations by the time it reached its destination. 1 out of 50 disappeared. 1 out of 20 was damaged. The process was slow, inefficient and cost a fortune - and was entirely different depending on the facility and the type of goods.

Similarly, before Standard Containers, by the time a software component ran in production, it had been individually built, configured, bundled, documented, patched, vendored, templated, tweaked and instrumented by 10 different people on 10 different computers. Builds failed, libraries conflicted, mirrors crashed, post-it notes were lost, logs were misplaced, cluster updates were half-broken. The process was slow, inefficient and cost a fortune - and was entirely different depending on the language and infrastructure provider.

## 5. Industrial-grade delivery

There are 17 million shipping containers in existence, packed with every physical good imaginable. Every single one of them can be loaded onto the same boats, by the same cranes, in the same facilities, and sent anywhere in the World with incredible efficiency. It is embarrassing to think that a 30 ton shipment of coffee can safely travel half-way across the World in *less time* than it takes a software team to deliver its code from one datacenter to another sitting 10 miles away.

With Standard Containers we can put an end to that embarrassment, by making INDUSTRIAL-GRADE DELIVERY of software a reality.

# Container format

This section defines a format for encoding a container as a *bundle* - a directory organized in a certain way, and containing all the necessary data and metadata for any compliant runtime to perform all standard operations against it. See also *[Mac OS application bundle*s](http://en.wikipedia.org/wiki/Bundle_%28OS_X%29) for a similar use of the term *bundle*.

The format does not define distribution. In other words, it only specifies how a container must be stored on a local filesystem, for consumption by a runtime. It does not specify how to transfer a container between computers, how to discover containers, or assign names or versions to them. Any distribution method capable of preserving the original layout of a container, as specified here, is considered compliant.

A standard container bundle is made of the following 4 parts:

* A top-level directory holding everything else

* One or more content directories

* Optional cryptographic signatures

* A configuration file

## 1 Directory layout

A Standard Container bundle is a directory containing all the content needed to load and run a container. This includes its configuration file, content directories, and cryptographic signatures. The main property of this directory layout is that it can be moved as a unit to another machine and run the same container.

One or more *content directories* may be adjacent to the configuration file. This at least includes the root filesystem (referenced in the configuration by the *rootfs* field), and any number of   and other related content (signatures, other configs, etc.). The interpretation of these resources is specified in the configuration.

```
/
!
-- config.json
!
--- rootfs1
!
--- rootfs2
```

The syntax and semantics for config.json are described in this specification.

## 2 Content directories

One or more content directories can be specified as root file systems for containers. They COULD be called rootfs..10^100 but SHALL be called whatever you want.

## 3 Cryptographic signatures

NOTE: I know this is sounding crazy, but it just might work! The main problem is that this is very slow. Every file in the container’s root filesystem must be read. It is, however, very flexible and quite portable. Some things to decide here:

* How carefully do we specify the digest file? (bad, but accurate name)

* Should we maintain a registry of "supported" signature schemes or should we let the world go wild? I have gpg example below but a tuf repo would be reasonable to implement below. Flexibility vs. interoperability.

To ensure that containers can be reliably transferred between implementations and machines, we define a flexible hashing and signature system that can be used to verify the unpacked content. The generation of signatures is separated into three different steps, known as "digest", “sign” and “verify”.

### Digest

The purpose of the "digest" step is to create a stable, summary of the content, invariant to irrelevant changes yet strong enough to avoid tampering. The algorithm for the digest is defined by an executable file, named “digest”, directly in the container directory. If such a file is present, it can be run with the container path as the first argument:
```
$ $CONTAINER_PATH/digest $CONTAINER_PATH
```
The nature of this executable is not important other than that it should run on a variety of systems with minimal dependencies. Typically, this can be a bourne shell script. The output of the script is left to the implementation but it is recommend that the output adhere to the following properties:

* The script itself should be included in the output in some way to avoid tampering

* The output should include the content, each filesystem path relative to the root and any other attributes that must be static for the container to operate correctly

* The output must be stable

* The only constraint is that the signatures directory should be ignored to avoid the act of signing preventing the content from being verified

The following is a naive example:
```
#!/usr/bin/env bash

set -e

# emit content for building a hash of the container filesystem.

content() {

	root=$1

	if [ -z "$root" ]; then

		echo "must specify root" 1>&2;

		exit 1;

	fi

	cd $root

	# emit the file paths, stat and their content hash

	find . -type f -not -path './signatures/*' -exec shasum -a256 {} \; | sort

	# emit the script itself to prevent tampering

	cat $scriptpath

}

scriptpath=$( cd $(dirname $0) ; pwd -P )/$(basename $0)

content $1 | shasum -a256
```

NOTE: The above is still pretty naive. It does not include permissions and users and other important aspects. This is just a demo. Part of the specification process would be producing a rock-solid, standard version of this script. It can be updated at any time and containers can use different versions depending on the use case.

Optionally, we can go with a set hashing approach. Here are a few requirements:

1. The hash will be made up of the hash of hashes of each resource in the container.

2. The order of the additions to the hash should be based on the lexical sort order of the relative container path of the resource.

3. Each resource should only be stat’ed and read once.

4. Unless specifically omitted, the hash should include the following resource types:

    1. files

    2. directories

    3. hard links

    4. soft links

    5. character devices

    6. block devices

    7. named fifo/pipes

    8. sockets

5. The hash of each resource must fix the following attributes:

    9. File path relative to root.

    10. File contents

    11. Owners (uid or names?)

    12. Groups (gid or names?)

    13. File mode/permissions

    14. xattr

    15. major/minor device numbers

    16. link target names

6. The hash should be re-calculable using information about only changed files.

### Sign

The output, known as the container’s *digest*, can be signed using any cryptography system (pgp, x509, jose). The result of which should be deposited in the container’s top-level signatures directory.

To sign the digest, we pipe the output to a cryptography tool. We can demonstrate the concept with gpg:
```
$ $CONTAINER_PATH/digest $CONTAINER_PATH | gpg --sign --detach-sign --armor > $CONTAINER_PATH/signatures/gpg/signature.asc
```
Notice that the signatures have been added to a directory, "gpg" to allow multiple signing systems to coexist.

### Verify

The container signature can be verified on another machine by piping the same command output, from the digest to a verification command.

Following from the gpg example:
```
$CONTAINER_PATH/digest $CONTAINER_PATH | gpg --verify $CONTAINER_PATH/signatures/gpg/signature.asc -
```

## 4. Configuration file

The container’s top-level directory MUST contain a configuration file called config.json. The configuration file MUST comply with the Open Container Configuration JSON Schema attached to this document. The latest version of the schema is available at [https://opencontainers.org/spec/schema.json](https://opencontainers.org/spec/schema.json)

### The configuration file contains metadata necessary to implement standard operations against the container. This includes processes to run, environment variables to inject, sandboxing features to use, etc.

Below is a detailed description of each field defined in the configuration format.

### Manifest version

The `version` element specifies the version of the OCF specification which the container complies with. If the container is compliant with multiple versions, it SHOULD advertise the most recent known version to be supported.

*Linux example*
```
{
    "version": "1",
```
### File system configuration

Each container has exactly one *root filesystem*, and any number of optional *mounted filesystems*. Both need to be declared in the manifest.

The rootfs string element specifies the path to the root file system for the container, relative to the path where the manifest is. A directory MUST exist at the relative path declared by the field. 

The readonlyRootfs is an optional boolean element which defaults to false. If it is true, access to the root file system MUST be read-only for all processes running inside it.  whether you want the root file system to be readonly or not for the processes running on it.

*Example (Linux)*
```
    "rootfs": "rootfs",

   "readonlyRootfs": true,
```
*Example (**Windows)*
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

#### Command-line arguments

#### TTY allocation

#### User ID

#### Environment variables

#### Working directory

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
### Resource Constraints

*Example*
```
    "cpus": 1.1,
    "memory": 1024,
    "hostname": "mrsdalloway",
```
The number of CPUs is specified as a positive decimal under the key cpus. 

The amount of memory allocated to this container is specified under the memory key, as an integer and is expressed in MBb.

If the cpu or memory requested are too high for the underlying environment capabilities, an error code NOT_ENOUGH_CPU or NOT_ENOUGH_MEM will be returned. 

hostname is a string specifying the hostname for that container as it is accessible to processes running in it.

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

## Security profiles

* Default security profile

* Privileged security profile

* Untrusted security profile

## Performance profiles

## Requiring native capabilities

### Linux

#### Linux Namespaces
```
    "namespaces": [
        "process",
        "network",
        "mount",
        "ipc",
        "uts"
    ],
```
Namespaces for the container are specified as an array of strings under the namespaces key. The list of constants that can be used is portable across operating systems. Here is a table mapping these names to native OS equivalent.

For Linux the mapping is

* process -> pid: the process ID number space is specific to the container, meaning that processes in different PID namespaces can have the same PID

* network -> network: the container will have an isolated network stack

* mount -> mnt container can only access mounts local to itself

* ipc -> ipc processes in the container can only communicate with other processes inside same container

* uts -> uts Hostname and NIS domain name are specific to the container

#### Linux Control groups

#### Linux Seccomp

#### Linux Process Capabilities
```
   "capabilities": [
        "AUDIT_WRITE",
        "KILL",
        "NET_BIND_SERVICE"
    ],
```    
capabilities is an array of Linux process capabilities. Valid values are the string after CAP_ for capabilities defined in http://linux.die.net/man/7/capabilities

#### SELinux

#### Apparmor

### Windows

### Solaris

### X86-64

**X86-32**

### ARM

## Machine-specific configuration
```
    "os": "linux",
    "arch": "amd64",
```
os specifies the operating system family this image must run on. 

arch specifies the instruction set for which the binaries in the image have been compiled. 

values for os and arch must be in the list specified by by the Go Language documentation for $GOOS and $GOARCH https://golang.org/doc/install/source#environment

OS or architecture specific settings can be added in the json file. They will be interpreted by the implementation depending on the os and arch values specified at the top of the manifest.

generate doc from code spec.go

# Standard operations (lifecycle)

## Create

Creates the container: file system, namespaces, cgroups, capabilities.

## Start (process)

Runs a process in a container. Can be invoked several times.

## Stop (process)

Not sure we need that from oc cli. Process is killed from the outside.

This event needs to be captured by oc to run onstop event handlers.

## Event Handlers

When these 3 events happen, oc can invoke event handlers specified in the config file.

The event handlers can be specific to inside or outside the container.

create: 2 events, outside the container, since the container has not been created yet. pre-create and post-create

start: 2 events outside, 2 events outside. outside-pre-start, inside-pre-start, inside-post-start, outside-post-start

stop: 1 event outside, post-stop

## Exec (covered by start?)

Covered in another tool.

## Restart (what does this do?)

## Seal

## Verify

## Destroy

# Execution environment

process environment, devices, generate it from licontainer/spec.md file, currently specifies it for linux.

# See also

http://flint.cs.yale.edu/cs422/doc/ELF_Format.pdf


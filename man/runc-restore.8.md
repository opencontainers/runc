% runc-restore "8"

# NAME
**runc-restore** - restore a container from a previous checkpoint

# SYNOPSIS
**runc restore** [_option_ ...] _container-id_

# DESCRIPTION
Restores the container instance from a previously performed **runc checkpoint**.

# OPTIONS
**--console-socket** _path_
: Path to an **AF_UNIX**  socket which will receive a file descriptor
referencing the master end of the console's pseudoterminal.  See
[docs/terminals](https://github.com/opencontainers/runc/blob/master/docs/terminals.md).

**--image-path** _path_
: Set path to get criu image files to restore from.

**--work-path** _path_
: Set path for saving criu work files and logs. The default is to reuse the
image files directory.

**--tcp-established**
: Allow checkpoint/restore of established TCP connections. See
[criu --tcp-establised option](https://criu.org/CLI/opt/--tcp-established).

**--ext-unix-sk**
: Allow checkpoint/restore of external unix sockets. See
[criu --ext-unix-sk option](https://criu.org/CLI/opt/--ext-unix-sk).

**--shell-job**
: Allow checkpoint/restore of shell jobs.

**--file-locks**
: Allow checkpoint/restore of file locks. See
[criu --file-locks option](https://criu.org/CLI/opt/--file-locks).

**--manage-cgroups-mode** **soft**|**full**|**strict**|**ignore**.
: Cgroups mode. Default is **soft**. See
[criu --manage-cgroups option](https://criu.org/CLI/opt/--manage-cgroups).

: In particular, to restore the container into a different cgroup,
**--manage-cgroups-mode ignore** must be used during both
**checkpoint** and **restore**, and the _container_id_ (or
**cgroupsPath** property in OCI config, if set) must be changed.

**--bundle**|**-b** _path_
: Path to the root of the bundle directory. Default is current directory.

**--detach**|**-d**
: Detach from the container's process.

**--pid-file** _path_
: Specify the file to write the initial container process' PID to.

**--no-subreaper**
: Disable the use of the subreaper used to reap reparented processes.

**--no-pivot**
: Do not use pivot root to jail process inside rootfs. This should not be used
except in exceptional circumstances, and may be unsafe from the security
standpoint.

**--empty-ns** _namespace_
: Create a _namespace_, but don't restore its properties. See
[criu --empty-ns option](https://criu.org/CLI/opt/--empty-ns).

**--auto-dedup**
: Enable auto deduplication of memory images. See
[criu --auto-dedup option](https://criu.org/CLI/opt/--auto-dedup).

**--lazy-pages**
: Use lazy migration mechanism. This requires a running **criu lazy-pages**
daemon. See [criu --lazy-pages option](https://criu.org/CLI/opt/--lazy-pages).

**--lsm-profile** _type_:_label_
: Specify an LSM profile to be used during restore. Here _type_ can either be
**apparamor** or **selinux**, and _label_ is a valid LSM label. For example,
**--lsm-profile "selinux:system_u:system_r:container_t:s0:c82,c137"**.
By default, the checkpointed LSM profile is used upon restore.

**--lsm-mount-context** _context_
: Specify an LSM mount context to be used during restore. Only mounts with an
existing context will have their context replaced. With this option it is
possible to change SELinux mount options. Instead of mounting with the
checkpointed context, the specified _context_ will be used.
For example, **--lsm-mount-context "system_u:object_r:container_file_t:s0:c82,c137"**.

# SEE ALSO
**criu**(8),
**runc-checkpoint**(8),
**runc**(8).

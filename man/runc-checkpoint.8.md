% runc-checkpoint "8"

# NAME
**runc-checkpoint** - checkpoint a running container

# SYNOPSIS
**runc checkpoint** [_option_ ...] _container-id_

# DESCRIPTION
The **checkpoint** command saves the state of the running container instance
with the help of **criu**(8) tool, to be restored later.

# OPTIONS
**--image-path** _path_
: Set path for saving criu image files. The default is *./checkpoint*.

**--work-path** _path_
: Set path for saving criu work files and logs. The default is to reuse the
image files directory.

**--parent-path** _path_
: Set path for previous criu image files, in pre-dump.

**--leave-running**
: Leave the process running after checkpointing.

**--tcp-established**
: Allow checkpoint/restore of established TCP connections. See
[criu --tcp-establised option](https://criu.org/CLI/opt/--tcp-established).

**--ext-unix-sk**
: Allow checkpoint/restore of external unix sockets. See
[criu --ext-unix-sk option](https://criu.org/CLI/opt/--ext-unix-sk).

**--shell-job**
: Allow checkpoint/restore of shell jobs.

**--lazy-pages**
: Use lazy migration mechanism. See
[criu --lazy-pages option](https://criu.org/CLI/opt/--lazy-pages).

**--status-fd** _fd_
: Pass a file descriptor _fd_ to **criu**. Once **lazy-pages** server is ready,
**criu** writes **\0** (a zero byte) to that _fd_. Used together with
**--lazy-pages**.

**--page-server** _IP-address_:_port_
: Start a page server at the specified _IP-address_ and _port_. This is used
together with **criu lazy-pages**. See
[criu lazy migration](https://criu.org/Lazy_migration).

**--file-locks**
: Allow checkpoint/restore of file locks. See
[criu --file-locks option](https://criu.org/CLI/opt/--file-locks).

**--pre-dump**
: Do a pre-dump, i.e. dump container's memory information only, leaving the
container running. See [criu iterative migration](https://criu.org/Iterative_migration).

**--manage-cgroups-mode** **soft**|**full**|**strict**|**ignore**.
: Cgroups mode. Default is **soft**. See
[criu --manage-cgroups option](https://criu.org/CLI/opt/--manage-cgroups).

**--empty-ns** _namespace_
: Checkpoint a _namespace_, but don't save its properties. See
[criu --empty-ns option](https://criu.org/CLI/opt/--empty-ns).

**--auto-dedup**
: Enable auto deduplication of memory images. See
[criu --auto-dedup option](https://criu.org/CLI/opt/--auto-dedup).

# SEE ALSO
**criu**(8),
**runc-restore**(8),
**runc**(8),
**criu**(8).

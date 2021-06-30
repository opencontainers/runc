% runc-run "8"

# NAME
**runc-run** - create and start a container

# SYNOPSIS
**runc run** [_option_ ...] _container-id_

# DESCRIPTION
The **run** command creates an instance of a container from a bundle, and
starts it.  You can think of **run** as a shortcut for **create** followed by
**start**.

# OPTIONS
**--bundle**|**-b** _path_
: Path to the root of the bundle directory. Default is current directory.

**--console-socket** _path_
: Path to an **AF_UNIX**  socket which will receive a file descriptor
referencing the master end of the console's pseudoterminal.  See
[docs/terminals](https://github.com/opencontainers/runc/blob/master/docs/terminals.md).

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

**--no-new-keyring**
: Do not create a new session keyring for the container. This will cause the
container to inherit the calling processes session key.

**--preserve-fds** _N_
: Pass _N_ additional file descriptors to the container (**stdio** +
**$LISTEN_FDS** + _N_ in total). Default is **0**.

**--keep**
: Keep container's state directory and cgroup. This can be helpful if a user
wants to check the state (e.g. of cgroup controllers) after the container has
exited. If this option is used, a manual **runc delete** is needed afterwards
to clean an exited container's artefacts.

# SEE ALSO

**runc**(8).

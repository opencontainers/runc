% runc-create "8"

# NAME
**runc-create** - create a container

# SYNOPSIS
**runc create** [_option_ ...] _container-id_

# DESCRIPTION
The **create** command creates an instance of a container from a bundle.
The bundle is a directory with a specification file named _config.json_,
and a root filesystem.

# OPTIONS

**--bundle**|**-b** _path_
: Path to the root of the bundle directory. Default is current directory.

**--console-socket** _path_
: Path to an **AF_UNIX**  socket which will receive a file descriptor
referencing the master end of the console's pseudoterminal.  See
[docs/terminals](https://github.com/opencontainers/runc/blob/master/docs/terminals.md).

**--pid-file** _path_
: Specify the file to write the initial container process' PID to.

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

# SEE ALSO

**runc-spec**(8),
**runc-start**(8),
**runc**(8).

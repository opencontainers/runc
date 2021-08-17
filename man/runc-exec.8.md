% runc-exec "8"

# NAME
**runc-exec** - execute new process inside the container

# SYNOPSIS
**runc exec** [_option_ ...] _container-id_ [--] _command_ [_arg_ ...]

**runc exec** [_option_ ...] **-p** _process.json_ _container-id_

# OPTIONS
**--console-socket** _path_
: Path to an **AF_UNIX**  socket which will receive a file descriptor
referencing the master end of the console's pseudoterminal.  See
[docs/terminals](https://github.com/opencontainers/runc/blob/master/docs/terminals.md).

**--cwd** _path_
: Change to _path_ in the container before executing the command.

**--env**|**-e** _name_=_value_
: Set an environment variable _name_ to _value_. Can be specified multiple times.

**--tty**|**-t**
: Allocate a pseudo-TTY.

**--user**|**-u** _uid_[:_gid_]
: Run the _command_ as a user (and, optionally, group) specified by _uid_ (and
_gid_).

**--additional-gids**|**-g** _gid_
: Add additional group IDs. Can be specified multiple times.

**--process**|**-p** _process.json_
: Instead of specifying all the exec parameters directly on the command line,
get them from a _process.json_, a JSON file containing the process
specification as defined by the
[OCI runtime spec](https://github.com/opencontainers/runtime-spec/blob/master/config.md#process).

**--detach**|**-d**
: Detach from the container's process.

**--pid-file** _path_
: Specify the file to write the container process' PID to.

**--process-label** _label_
: Set the asm process label for the process commonly used with **selinux**(7).

**--apparmor** _profile_
: Set the **apparmor**(7) _profile_ for the process.

**--no-new-privs**
: Set the "no new privileges" value for the process.

**--cap** _cap_
: Add a capability to the bounding set for the process. Can be specified
multiple times.

**--preserve-fds** _N_
: Pass _N_ additional file descriptors to the container (**stdio** +
**$LISTEN_FDS** + _N_ in total). Default is **0**.

**--ignore-paused**
: Allow exec in a paused container. By default, if a container is paused,
**runc exec** errors out; this option can be used to override it.
A paused container needs to be resumed for the exec to complete.

**--cgroup** _path_ | _controller_[,_controller_...]:_path_
: Execute a process in a sub-cgroup. If the specified cgroup does not exist, an
error is returned. Default is empty path, which means to use container's top
level cgroup.
: For cgroup v1 only, a particular _controller_ (or multiple comma-separated
controllers) can be specified, and the option can be used multiple times to set
different paths for different controllers.
: Note for cgroup v2, in case the process can't join the top level cgroup,
**runc exec** fallback is to try joining the cgroup of container's init.
This fallback can be disabled by using **--cgroup /**.

# EXIT STATUS

Exits with a status of _command_ (unless **-d** is used), or **255** if
an error occurred.

# EXAMPLES
If the container can run **ps**(1) command, the following
will output a list of processes running in the container:

	# runc exec <container-id> ps

# SEE ALSO

**runc**(8).

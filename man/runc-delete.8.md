% runc-delete "8"

# NAME
**runc-delete** - delete any resources held by the container

# SYNOPSIS
**runc delete** [**--force**|**-f**] _container-id_

# OPTIONS
**--force**|**-f**
: Forcibly delete the running container, using **SIGKILL** **signal**(7)
to stop it first.

# EXAMPLES
If the container id is **ubuntu01** and **runc list** currently shows
its status as **stopped**, the following will delete resources held for
**ubuntu01**, removing it from the **runc list**:

	# runc delete ubuntu01

# SEE ALSO

**runc-kill**(8),
**runc**(8).

% runc-pause "8"

# NAME
**runc-pause** - suspend all processes inside the container

# SYNOPSIS
**runc pause** _container-id_

# DESCRIPTION
The **pause** command suspends all processes in the instance of the container
identified by _container-id_.

Use **runc list** to identify instances of containers and their current status.

# SEE ALSO
**runc-list**(8),
**runc-resume**(8),
**runc**(8).

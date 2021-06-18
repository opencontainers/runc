% runc-ps "8"

# NAME
**runc-ps** - display the processes inside a container

# SYNOPSIS
**runc ps** [_option_ ...] _container-id_ [_ps-option_ ...]

# DESCRIPTION
The command **ps** is a wrapper around the stock **ps**(1) utility,
which filters its output to only contain processes belonging to a specified
_container-id_. Therefore, the PIDs shown are the host PIDs.

Any **ps**(1) options can be used, but some might break the filtering.
In particular, if PID column is not available, an error is returned,
and if there are columns with values containing spaces before the PID
column, the result is undefined.

# OPTIONS
**--format**|**-f** **table**|**json**
: Output format. Default is **table**. The **json** format shows a mere array
of PIDs belonging to a container; if used, all **ps** options are gnored.

# SEE ALSO
**runc-list**(8),
**runc**(8).

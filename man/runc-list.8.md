% runc-list "8"

# NAME
**runc-list** - lists containers

# SYNOPSIS
**runc list** [_option_ ...]

# DESCRIPTION

The **list** commands lists containers. Note that a global **--root**
option can be specified to change the default root. For the description
of **--root**, see **runc**(8).

# OPTIONS
**--format**|**-f** **table**|**json**
: Specify the format. Default is **table**. The **json** format provides
more details.

**--quiet**|**-q**
: Only display container IDs.

# EXAMPLES
To list containers created with the default root:

	# runc list

To list containers in a human-readable JSON (with the help of **jq**(1)
utility):

	# runc list -f json | jq

To list containers created with the root of **/tmp/myroot**:

	# runc --root /tmp/myroot

# SEE ALSO

**runc**(8).

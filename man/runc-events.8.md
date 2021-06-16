% runc-events "8"

# NAME
**runc-events** - display container events and statistics.

# SYNOPSIS
**runc events** [_option_ ...] _container-id_

# DESCRIPTION
The **events** command displays information about the container. By default,
it works continuously, displaying stats every 5 seconds, and container events
as they occur.

# OPTIONS
**--interval** _time_
: Set the stats collection interval. Default is **5s**.

**--stats**
: Show the container's stats once then exit.

# SEE ALSO

**runc**(8).

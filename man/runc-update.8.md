% runc-update "8"

# NAME
**runc-update** - update running container resource constraints

# SYNOPSIS
**runc update** [_option_ ...] _container-id_

**runc update** **-r** _resources.json_|**-**  _container-id_

# DESCRIPTION
The **update** command change the resource constraints of a running container
instance.

The resources can be set using options, or, if **-r** is used, parsed from JSON
provided as a file or from stdin.

In case **-r** is used, the JSON format is like this:

	{
		"memory": {
			"limit": 0,
				"reservation": 0,
				"swap": 0,
				"kernel": 0,
				"kernelTCP": 0
		},
			"cpu": {
				"shares": 0,
				"quota": 0,
				"burst": 0,
				"period": 0,
				"realtimeRuntime": 0,
				"realtimePeriod": 0,
				"cpus": "",
				"mems": ""
			},
			"blockIO": {
				"blkioWeight": 0
			}
	}

# OPTIONS
**--resources**|**-r** _resources.json_
: Read the new resource limits from _resources.json_. Use **-** to read from
stdin. If this option is used, all other options are ignored.

**--blkio-weight** _weight_
: Set a new io weight.

**--cpu-period** _num_
: Set CPU CFS period to be used for hardcapping (in microseconds)

**--cpu-quota** _num_
: Set CPU usage limit within a given period (in microseconds).

**--cpu-burst** _num_
: Set CPU burst limit within a given period (in microseconds).

**--cpu-rt-period** _num_
: Set CPU realtime period to be used for hardcapping (in microseconds).

**--cpu-rt-runtime** _num_
: Set CPU realtime hardcap limit (in usecs). Allowed cpu time in a given period.

**--cpu-share** _num_
: Set CPU shares (relative weight vs. other containers).

**--cpuset-cpus** _list_
: Set CPU(s) to use. The _list_ can contain commas and ranges. For example:
**0-3,7**.

**--cpuset-mems** _list_
: Set memory node(s) to use. The _list_ format is the same as for
**--cpuset-cpus**.

**--memory** _num_
: Set memory limit to _num_ bytes.

**--memory-reservation** _num_
: Set memory reservation, or soft limit, to _num_ bytes.

**--memory-swap** _num_
: Set total memory + swap usage to _num_ bytes. Use **-1** to unset the limit
(i.e. use unlimited swap).

**--pids-limit** _num_
: Set the maximum number of processes allowed in the container.

**--l3-cache-schema** _value_
: Set the value for Intel RDT/CAT L3 cache schema.

**--mem-bw-schema** _value_
: Set the Intel RDT/MBA memory bandwidth schema.

# SEE ALSO

**runc**(8).

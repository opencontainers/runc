# NAME
   runc update - update container resource constraints

# SYNOPSIS
   runc update [command options] <container-id>

# DESCRIPTION
   The data can be read from a file or the standard input, the
accepted format is as follow (unchanged values can be omitted):

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
       "period": 0,
       "cpus": "",
       "mems": ""
     },
     "blockIO": {
       "blkioWeight": 0
     },
   }

Note: if data is to be read from a file or the standard input, all
other options are ignored.

# OPTIONS
   --resources, -r         path to the file containing the resources to update or '-' to read from the standard input.
   --blkio-weight "0"      Specifies per cgroup weight, range is from 10 to 1000.
   --cpu-period            CPU period to be used for hardcapping (in usecs). 0 to use system default.
   --cpu-quota             CPU hardcap limit (in usecs). Allowed cpu time in a given period.
   --cpu-share             CPU shares (relative weight vs. other containers)
   --cpuset-cpus           CPU(s) to use
   --cpuset-mems           Memory node(s) to use
   --kernel-memory         Kernel memory limit (in bytes)
   --kernel-memory-tcp     Kernel memory limit (in bytes) for tcp buffer
   --memory                Memory limit (in bytes)
   --memory-reservation    Memory reservation or soft_limit (in bytes)
   --memory-swap           Total memory usage (memory + swap); set `-1` to enable unlimited swap

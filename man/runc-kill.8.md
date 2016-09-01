# NAME
   runc kill - kill sends the specified signal (default: SIGTERM) to any of the container's processes (default: init process)

# SYNOPSIS
   runc kill [command options] <container-id> <signal>

Where "<container-id>" is the name for the instance of the container and
"<signal>" is the signal to be sent to the process of the container.

# EXAMPLE

For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runc kill ubuntu01 KILL

OPTIONS:
   --pid value, -p value  specify the pid to which process the signal would be sent (default: init process) (default: 0)

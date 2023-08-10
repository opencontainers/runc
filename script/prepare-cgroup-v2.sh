#!/bin/bash
#
# This script is used from ../Dockerfile as the ENTRYPOINT. It sets up cgroup
# delegation for cgroup v2 to make sure runc tests can be properly run inside
# a container.

# Only do this for cgroup v2.
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
	set -x
	# Move the current process to a sub-cgroup.
	mkdir /sys/fs/cgroup/init
	echo 0 >/sys/fs/cgroup/init/cgroup.procs
	# Enable all controllers.
	sed 's/\b\w/+\0/g' <"/sys/fs/cgroup/cgroup.controllers" >"/sys/fs/cgroup/cgroup.subtree_control"
fi

exec "$@"

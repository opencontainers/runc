#!/bin/bash
#
# This script is used from ../Dockerfile as the ENTRYPOINT. It sets up cgroup
# delegation for cgroup v2 to make sure runc tests can be properly run inside
# a container. It can also be run as a separate command in an already running
# container.

# Only do this for cgroup v2.
if [ -f /sys/fs/cgroup/cgroup.controllers ]; then
	set -x
	# Move all processes to a sub-cgroup, as controllers can't be enabled
	# in a cgroup which has processes in it.
	mkdir -p /sys/fs/cgroup/init
	mapfile -t pids </sys/fs/cgroup/cgroup.procs
	for pid in "${pids[@]}"; do
		# A process may have exited already, so ignore errors.
		echo "$pid" >/sys/fs/cgroup/init/cgroup.procs 2>/dev/null || true
	done
	# Enable all controllers.
	sed 's/\b\w/+\0/g' <"/sys/fs/cgroup/cgroup.controllers" >"/sys/fs/cgroup/cgroup.subtree_control"
fi

exec "$@"

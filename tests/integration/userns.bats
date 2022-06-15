#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Prepare source folders for bind mount
	mkdir -p source-{accessible,inaccessible-1,inaccessible-2}/dir
	touch source-{accessible,inaccessible-1,inaccessible-2}/dir/foo.txt

	# Permissions only to the owner, it is inaccessible to group/others
	chmod 700 source-inaccessible-{1,2}

	mkdir -p rootfs/{proc,sys,tmp}
	mkdir -p rootfs/tmp/mount-{1,2}

	if [ $EUID -eq 0 ]; then
		update_config ' .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
			| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}] '
	fi
}

function teardown() {
	teardown_bundle
}

@test "userns with simple mount" {
	update_config ' .process.args += ["-c", "stat /tmp/mount-1/foo.txt"]
		| .mounts += [{"source": "source-accessible/dir", "destination": "/tmp/mount-1", "options": ["bind"]}] '

	runc run test_busybox
	[ "$status" -eq 0 ]
}

# We had bugs where 1 mount worked but not 2+, test with 2 as it is a more
# general case.
@test "userns with 2 inaccessible mounts" {
	update_config '   .process.args += ["-c", "stat /tmp/mount-1/foo.txt /tmp/mount-2/foo.txt"]
			| .mounts += [	{ "source": "source-inaccessible-1/dir", "destination": "/tmp/mount-1", "options": ["bind"] },
			                { "source": "source-inaccessible-2/dir", "destination": "/tmp/mount-2", "options": ["bind"] }
			           ]'

	# When not running rootless, this should work: while
	# "source-inaccessible-1" can't be read by the uid in the userns, the fd
	# is opened before changing to the userns and sent over via SCM_RIGHTs
	# (with env var _LIBCONTAINER_MOUNT_FDS). Idem for
	# source-inaccessible-2.
	# On rootless, the owner is the same so it is accessible.
	runc run test_busybox
	[ "$status" -eq 0 ]
}

# exec + bindmounts + user ns is a special case in the code. Test that it works.
@test "userns with inaccessible mount + exec" {
	update_config ' .mounts += [ 	{ "source": "source-inaccessible-1/dir", "destination": "/tmp/mount-1", "options": ["bind"] },
					{ "source": "source-inaccessible-2/dir", "destination": "/tmp/mount-2", "options": ["bind"] }
			         ]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox stat /tmp/mount-1/foo.txt /tmp/mount-2/foo.txt
	[ "$status" -eq 0 ]
}

# Issue fixed by https://github.com/opencontainers/runc/pull/3510.
@test "userns with bind mount before a cgroupfs mount" {
	# This can only be reproduced on cgroup v1 (and no cgroupns) due to the
	# way it is mounted in such case (a bunch of of bind mounts).
	requires cgroups_v1

	# Add a bind mount right before the /sys/fs/cgroup mount,
	# and make sure cgroupns is not enabled.
	update_config '	  .mounts |= map(if .destination == "/sys/fs/cgroup" then ({"source": "source-accessible/dir", "destination": "/tmp/mount-1", "options": ["bind"]}, .) else . end)
			| .linux.namespaces -= [{"type": "cgroup"}]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# Make sure this is real cgroupfs.
	runc exec test_busybox cat /sys/fs/cgroup/{pids,memory}/tasks
	[ "$status" -eq 0 ]
}

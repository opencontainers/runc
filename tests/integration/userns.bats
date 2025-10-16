#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Prepare source folders for bind mount
	mkdir -p source-{accessible,inaccessible-1,inaccessible-2}/dir
	touch source-{accessible,inaccessible-1,inaccessible-2}/dir/foo.txt

	# Permissions only to the owner, it is inaccessible to group/others
	chmod 700 source-inaccessible-{1,2}

	mkdir -p rootfs/tmp/mount-{1,2}

	to_umount_list="$(mktemp "$BATS_RUN_TMPDIR/userns-mounts.XXXXXX")"
	if [ $EUID -eq 0 ]; then
		update_config ' .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65534}]
			| .linux.gidMappings += [{"hostID": 200000, "containerID": 0, "size": 65534}] '
		remap_rootfs
	fi
}

function teardown() {
	teardown_bundle

	if [ -v to_umount_list ]; then
		while read -r mount_path; do
			umount -l "$mount_path" || :
			rm -rf "$mount_path"
		done <"$to_umount_list"
		rm -f "$to_umount_list"
		unset to_umount_list
	fi
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

@test "userns join other container userns" {
	# Create a detached container with the id-mapping we want.
	update_config '.process.args = ["sleep", "infinity"]'
	runc run -d --console-socket "$CONSOLE_SOCKET" target_userns
	[ "$status" -eq 0 ]

	# Configure our container to attach to the first container's userns.
	target_pid="$(__runc state target_userns | jq .pid)"
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "/proc/'"$target_pid"'/ns/" + .type) else . end)
		| del(.linux.uidMappings)
		| del(.linux.gidMappings)'
	runc run -d --console-socket "$CONSOLE_SOCKET" in_userns
	[ "$status" -eq 0 ]

	runc exec in_userns cat /proc/self/uid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+100000\s+65534$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi

	runc exec in_userns cat /proc/self/gid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+200000\s+65534$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi
}

# issue: https://github.com/opencontainers/runc/issues/4466
@test "userns join other container userns[selinux enabled]" {
	if ! selinuxenabled; then
		skip "requires SELinux enabled and in enforcing mode"
	fi
	# Create a detached container with the id-mapping we want.
	update_config '.process.args = ["sleep", "infinity"]'
	runc run -d --console-socket "$CONSOLE_SOCKET" target_userns
	[ "$status" -eq 0 ]

	# Configure our container to attach to the first container's userns.
	target_pid="$(__runc state target_userns | jq .pid)"
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "/proc/'"$target_pid"'/ns/" + .type) else . end)
		| del(.linux.uidMappings)
		| del(.linux.gidMappings)
		| .linux.mountLabel="system_u:object_r:container_file_t:s0:c344,c805"'
	runc run -d --console-socket "$CONSOLE_SOCKET" in_userns
	[ "$status" -eq 0 ]
}

@test "userns join other container userns [bind-mounted nsfd]" {
	requires root

	# Create a detached container with the id-mapping we want.
	update_config '.process.args = ["sleep", "infinity"]'
	runc run -d --console-socket "$CONSOLE_SOCKET" target_userns
	[ "$status" -eq 0 ]

	# Bind-mount the first containers userns nsfd to a different path, to
	# exercise the non-fast-path (where runc has to join the userns to get the
	# mappings).
	target_pid="$(__runc state target_userns | jq .pid)"
	userns_path=$(mktemp "$BATS_RUN_TMPDIR/userns.XXXXXX")
	mount --bind "/proc/$target_pid/ns/user" "$userns_path"
	echo "$userns_path" >>"$to_umount_list"

	# Configure our container to attach to the first container's userns.
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "'"$userns_path"'") else . end)
		| del(.linux.uidMappings)
		| del(.linux.gidMappings)'
	runc run -d --console-socket "$CONSOLE_SOCKET" in_userns
	[ "$status" -eq 0 ]

	runc exec in_userns cat /proc/self/uid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+100000\s+65534$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi

	runc exec in_userns cat /proc/self/gid_map
	[ "$status" -eq 0 ]
	if [ $EUID -eq 0 ]; then
		grep -E '^\s+0\s+200000\s+65534$' <<<"$output"
	else
		grep -E '^\s+0\s+'$EUID'\s+1$' <<<"$output"
	fi
}

# <https://github.com/opencontainers/runc/issues/4390>
@test "userns join external namespaces [wrong userns owner]" {
	requires root

	# Create an external user namespace for us to join. It seems on some
	# operating systems (AlmaLinux in particular) "unshare -U" will
	# automatically use an identity mapping (which breaks this test) so we need
	# to use runc to create the userns.
	update_config '.process.args = ["sleep", "infinity"]'
	runc run -d --console-socket "$CONSOLE_SOCKET" target_userns
	[ "$status" -eq 0 ]

	# Bind-mount the first containers userns nsfd to a different path, to
	# exercise the non-fast-path (where runc has to join the userns to get the
	# mappings).
	userns_pid="$(__runc state target_userns | jq .pid)"
	userns_path="$(mktemp "$BATS_RUN_TMPDIR/userns.XXXXXX")"
	mount --bind "/proc/$userns_pid/ns/user" "$userns_path"
	echo "$userns_path" >>"$to_umount_list"

	# Kill the container -- we have the userns bind-mounted.
	runc delete -f target_userns
	[ "$status" -eq 0 ]

	# Configure our container to attach to the external userns.
	update_config '.linux.namespaces |= map(if .type == "user" then (.path = "'"$userns_path"'") else . end)
		| del(.linux.uidMappings)
		| del(.linux.gidMappings)'

	# Also create a network namespace that *is not owned* by the above userns.
	# NOTE: Having no permissions in a namespaces makes it necessary to modify
	# the config so that we don't get mount errors (for reference: no netns
	# permissions == no sysfs mounts, no pidns permissions == no procfs mounts,
	# no utsns permissions == no sethostname(2), no ipc permissions == no
	# mqueue mounts, etc).
	netns_path="$(mktemp "$BATS_RUN_TMPDIR/netns.XXXXXX")"
	unshare -i -- mount --bind "/proc/self/ns/net" "$netns_path"
	echo "$netns_path" >>"$to_umount_list"
	# Configure our container to attach to the external netns.
	update_config '.linux.namespaces |= map(if .type == "network" then (.path = "'"$netns_path"'") else . end)'

	# Convert sysfs mounts to a bind-mount from the host, to avoid permission
	# issues due to the netns setup we have.
	update_config '.mounts |= map((select(.type == "sysfs") | { "source": "/sys", "destination": .destination, "type": "bind", "options": ["rbind"] }) // .)'

	# Create a detached container to verify the namespaces are correct.
	update_config '.process.args = ["sleep", "infinity"]'
	runc --debug run -d --console-socket "$CONSOLE_SOCKET" ctr
	[ "$status" -eq 0 ]

	userns_id="user:[$(stat -c "%i" "$userns_path")]"
	netns_id="net:[$(stat -c "%i" "$netns_path")]"

	runc exec ctr readlink /proc/self/ns/user
	[ "$status" -eq 0 ]
	[[ "$output" == "$userns_id" ]]

	runc exec ctr readlink /proc/self/ns/net
	[ "$status" -eq 0 ]
	[[ "$output" == "$netns_id" ]]
}

@test "userns with network interface" {
	requires root

	# Create a dummy interface to move to the container.
	ip link add dummy0 type dummy

	update_config ' .linux.netDevices |= {"dummy0": {} }
		| .process.args |= ["ip", "address", "show", "dev", "dummy0"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# The interface is virtual and should not exist because
	# is deleted during the namespace cleanup.
	run ! ip link del dummy0
}

@test "userns with network interface renamed" {
	requires root

	# Create a dummy interface to move to the container.
	ip link add dummy0 type dummy

	update_config ' .linux.netDevices |= { "dummy0": { "name" : "ctr_dummy0" } }
		| .process.args |= ["ip", "address", "show", "dev", "ctr_dummy0"]'

	runc run test_busybox
	[ "$status" -eq 0 ]

	# The interface is virtual and should not exist because
	# is deleted during the namespace cleanup.
	run ! ip link del dummy0
}

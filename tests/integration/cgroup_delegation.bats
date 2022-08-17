#!/usr/bin/env bats

load helpers

function teardown() {
	teardown_bundle
}

function setup() {
	requires root cgroups_v2 systemd

	setup_busybox

	# chown test temp dir to allow host user to read it
	chown 100000 "$ROOT"

	# chown rootfs to allow host user to mkdir mount points
	chown 100000 "$ROOT"/bundle/rootfs

	set_cgroups_path

	# configure a user namespace
	update_config '   .linux.namespaces += [{"type": "user"}]
			| .linux.uidMappings += [{"hostID": 100000, "containerID": 0, "size": 65536}]
			| .linux.gidMappings += [{"hostID": 100000, "containerID": 0, "size": 65536}]
			'
}

@test "runc exec (cgroup v2, ro cgroupfs, new cgroupns) does not chown cgroup" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroup_chown
	[ "$status" -eq 0 ]

	runc exec test_cgroup_chown sh -c "stat -c %U /sys/fs/cgroup"
	[ "$status" -eq 0 ]
	[ "$output" = "nobody" ] # /sys/fs/cgroup owned by unmapped user
}

@test "runc exec (cgroup v2, rw cgroupfs, inherit cgroupns) does not chown cgroup" {
	set_cgroup_mount_writable

	# inherit cgroup namespace (remove cgroup from namespaces list)
	update_config '.linux.namespaces |= map(select(.type != "cgroup"))'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroup_chown
	[ "$status" -eq 0 ]

	runc exec test_cgroup_chown sh -c "stat -c %U /sys/fs/cgroup"
	[ "$status" -eq 0 ]
	[ "$output" = "nobody" ] # /sys/fs/cgroup owned by unmapped user
}

@test "runc exec (cgroup v2, rw cgroupfs, new cgroupns) does chown cgroup" {
	set_cgroup_mount_writable

	runc run -d --console-socket "$CONSOLE_SOCKET" test_cgroup_chown
	[ "$status" -eq 0 ]

	runc exec test_cgroup_chown sh -c "stat -c %U /sys/fs/cgroup"
	[ "$status" -eq 0 ]
	[ "$output" = "root" ] # /sys/fs/cgroup owned by root (of user namespace)
}

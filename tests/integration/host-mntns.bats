#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_busybox
	update_config '	  .linux.namespaces -= [{"type": "mount"}]
			| .linux.maskedPaths = []
			| .linux.readonlyPaths = []
			| .root.readonly = false'
}

function teardown() {
	# Remove the rootfs mount made by a test.
	if [ -v ROOT ] && mountpoint -q "$ROOT"/bundle/rootfs; then
		umount -R --lazy "$ROOT"/bundle/rootfs
	fi
	teardown_bundle
}

# This test goes first, since without the fix, any test which runs
# a container in the host mount namespace changes the host mounts
# propagation, so the bug would go unnoticed.
@test "runc run [host mount ns] must not change host mounts propagation" {
	update_config '.process.args = ["true"]'

	# Check / and the mount the container bundle resides on.
	before=$(findmnt -n -o TARGET,PROPAGATION / && findmnt -n -o TARGET,PROPAGATION -T "$ROOT/bundle")
	run -0 runc run test_host_mntns
	after=$(findmnt -n -o TARGET,PROPAGATION / && findmnt -n -o TARGET,PROPAGATION -T "$ROOT/bundle")
	echo "before: $before"
	echo "after: $after"
	[ "$before" = "$after" ]
}

@test "runc run [host mount ns + hooks]" {
	update_config '	  .process.args = ["/bin/echo", "Hello World"]
			| .hooks |= . + {"createRuntime": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", "touch createRuntimeHook.$$"]}]}'
	run -0 runc run test_host_mntns
	run -0 runc delete -f test_host_mntns

	# There should be one such file.
	run -0 ls createRuntimeHook.*
	[ "$(echo "$output" | wc -w)" -eq 1 ]
}

# https://github.com/opencontainers/runc/issues/2095
@test "runc delete [host mount ns] unmounts container mounts" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_host_mntns
	testcontainer test_host_mntns running

	# Container rootfs and its mounts are visible on the host.
	mountpoint -q rootfs
	mountpoint -q rootfs/proc

	run -0 runc delete -f test_host_mntns
	run ! mountpoint -q rootfs/proc
	run ! mountpoint -q rootfs
}

@test "runc run [host mount ns] unmounts container mounts on failure" {
	update_config '.hooks |= . + {"createRuntime": [{"path": "/bin/false"}]}'

	run ! runc run test_host_mntns
	run ! mountpoint -q rootfs/proc
	run ! mountpoint -q rootfs
}

@test "runc delete [host mount ns] keeps rootfs mounted by user" {
	# The rootfs is a mount point before the container is created
	# (and, if the bundle resides on a shared mount, it is a peer
	# of that mount).
	mount --bind rootfs rootfs

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_host_mntns
	testcontainer test_host_mntns running
	mountpoint -q rootfs/proc

	run -0 runc delete -f test_host_mntns
	run ! mountpoint -q rootfs/proc
	# The user mount must be kept intact.
	mountpoint -q rootfs
}

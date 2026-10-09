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
	[ ! -v ROOT ] && return 0 # nothing to teardown

	# XXX runc does not unmount a container which
	# shares mount namespace with the host.
	umount -R --lazy "$ROOT"/bundle/rootfs

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

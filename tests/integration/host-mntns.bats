#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_busybox
}

function teardown() {
	[ ! -v ROOT ] && return 0 # nothing to teardown

	# XXX runc does not unmount a container which
	# shares mount namespace with the host.
	umount -R --lazy "$ROOT"/bundle/rootfs

	teardown_bundle
}

@test "runc run [host mount ns + hooks]" {
	update_config '	  .process.args = ["/bin/echo", "Hello World"]
			| .hooks |= . + {"createRuntime": [{"path": "/bin/sh", "args": ["/bin/sh", "-c", "touch createRuntimeHook.$$"]}]}
			| .linux.namespaces -= [{"type": "mount"}]
			| .linux.maskedPaths = []
			| .linux.readonlyPaths = []'
	runc run test_host_mntns
	[ "$status" -eq 0 ]
	runc delete -f test_host_mntns

	# There should be one such file.
	run -0 ls createRuntimeHook.*
	[ "$(echo "$output" | wc -w)" -eq 1 ]
}

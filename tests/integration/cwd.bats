#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

# Test case for https://github.com/opencontainers/runc/pull/2086
@test "runc exec --user with no access to cwd" {
	requires root

	chown 42 rootfs/root
	chmod 700 rootfs/root

	update_config '	  .process.cwd = "/root"
			| .process.user.uid = 42
			| .process.args |= ["sleep", "1h"]'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec --user 0 test_busybox true
	[ "$status" -eq 0 ]
}

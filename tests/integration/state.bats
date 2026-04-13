#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "state (kill + delete)" {
	runc state test_busybox
	[ "$status" -ne 0 ]

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	runc kill test_busybox KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped

	runc delete test_busybox
	[ "$status" -eq 0 ]

	runc state test_busybox
	[ "$status" -ne 0 ]
}

@test "state (pause + resume)" {
	# XXX: pause and resume require cgroups.
	requires root

	runc state test_busybox
	[ "$status" -ne 0 ]

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	runc pause test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox paused

	runc resume test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

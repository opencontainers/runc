#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "kill detached busybox" {
	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# check state
	testcontainer test_busybox running

	runc kill test_busybox KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped

	# we should ensure kill work after the container stopped
	runc kill -a test_busybox 0
	[ "$status" -eq 0 ]

	runc delete test_busybox
	[ "$status" -eq 0 ]
}

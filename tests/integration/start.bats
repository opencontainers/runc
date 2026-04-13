#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc start" {
	runc create --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	runc start test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]
}

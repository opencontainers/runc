#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc start" {
	run -0 runc create --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	run -0 runc start test_busybox

	testcontainer test_busybox running

	run -0 runc delete --force test_busybox

	run ! runc state test_busybox
}

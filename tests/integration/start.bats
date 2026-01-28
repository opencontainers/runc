#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc start" {
	runc -0 create --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox created

	runc -0 start test_busybox

	testcontainer test_busybox running

	runc -0 delete --force test_busybox

	runc ! state test_busybox
}

#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "state (kill + delete)" {
	runc ! state test_busybox

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped

	runc -0 delete test_busybox

	runc ! state test_busybox
}

@test "state (pause + resume)" {
	# XXX: pause and resume require cgroups.
	requires root

	runc ! state test_busybox

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 pause test_busybox

	testcontainer test_busybox paused

	runc -0 resume test_busybox

	testcontainer test_busybox running
}

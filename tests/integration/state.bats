#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "state (kill + delete)" {
	run ! runc state test_busybox

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	run -0 runc kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped

	run -0 runc delete test_busybox

	run ! runc state test_busybox
}

@test "state (pause + resume)" {
	# XXX: pause and resume require cgroups.
	requires root

	run ! runc state test_busybox

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	run -0 runc pause test_busybox

	testcontainer test_busybox paused

	run -0 runc resume test_busybox

	testcontainer test_busybox running
}

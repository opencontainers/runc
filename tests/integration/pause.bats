#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc pause and resume" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		set_cgroups_path
	fi

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	run -0 runc pause test_busybox

	testcontainer test_busybox paused

	run -0 runc resume test_busybox

	testcontainer test_busybox running
}

@test "runc pause and resume with nonexist container" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		set_cgroups_path
	fi

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	run -0 runc pause test_busybox
	run ! runc pause nonexistent

	testcontainer test_busybox paused

	run -0 runc resume test_busybox
	run ! runc resume nonexistent

	testcontainer test_busybox running

	run -0 runc delete --force test_busybox

	run ! runc state test_busybox
}

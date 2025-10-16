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

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 pause test_busybox

	testcontainer test_busybox paused

	runc -0 resume test_busybox

	testcontainer test_busybox running
}

@test "runc pause and resume with nonexist container" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		set_cgroups_path
	fi

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	testcontainer test_busybox running

	runc -0 pause test_busybox
	runc ! pause nonexistent

	testcontainer test_busybox paused

	runc -0 resume test_busybox
	runc ! resume nonexistent

	testcontainer test_busybox running

	runc delete --force test_busybox

	runc ! state test_busybox
}

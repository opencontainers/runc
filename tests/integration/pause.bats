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

@test "runc pause and resume with nonexist container" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		set_cgroups_path
	fi

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	runc pause test_busybox
	[ "$status" -eq 0 ]
	runc pause nonexistent
	[ "$status" -ne 0 ]

	testcontainer test_busybox paused

	runc resume test_busybox
	[ "$status" -eq 0 ]
	runc resume nonexistent
	[ "$status" -ne 0 ]

	testcontainer test_busybox running

	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]
}

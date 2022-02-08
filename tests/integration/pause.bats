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

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	# pause busybox
	runc pause test_busybox
	[ "$status" -eq 0 ]

	# test state of busybox is paused
	testcontainer test_busybox paused

	# resume busybox
	runc resume test_busybox
	[ "$status" -eq 0 ]

	# test state of busybox is back to running
	testcontainer test_busybox running
}

@test "runc pause and resume with nonexist container" {
	requires cgroups_freezer
	if [ $EUID -ne 0 ]; then
		requires rootless_cgroup
		set_cgroups_path
	fi

	# run test_busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	# pause test_busybox and nonexistent container
	runc pause test_busybox
	[ "$status" -eq 0 ]
	runc pause nonexistent
	[ "$status" -ne 0 ]

	# test state of test_busybox is paused
	testcontainer test_busybox paused

	# resume test_busybox and nonexistent container
	runc resume test_busybox
	[ "$status" -eq 0 ]
	runc resume nonexistent
	[ "$status" -ne 0 ]

	# test state of test_busybox is back to running
	testcontainer test_busybox running

	# delete test_busybox
	runc delete --force test_busybox

	runc state test_busybox
	[ "$status" -ne 0 ]
}

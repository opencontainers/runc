#!/usr/bin/env bats

load helpers

function setup() {
	# ps requires cgroups
	[ $EUID -ne 0 ] && requires rootless_cgroup

	setup_busybox

	# Rootless does not have default cgroup path.
	[ $EUID -ne 0 ] && set_cgroups_path

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running
}

function teardown() {
	teardown_bundle
}

@test "ps" {
	runc -0 ps test_busybox
	[[ "$output" =~ UID\ +PID\ +PPID\ +C\ +STIME\ +TTY\ +TIME\ +CMD+ ]]
	[[ "$output" == *"$(id -un 2>/dev/null)"*[0-9]* ]]
}

@test "ps -f json" {
	runc -0 ps -f json test_busybox
	[[ "$output" =~ [0-9]+ ]]
}

@test "ps -e -x" {
	runc -0 ps test_busybox -e -x
	[[ "$output" =~ \ +PID\ +TTY\ +STAT\ +TIME\ +COMMAND+ ]]
	[[ "$output" =~ [0-9]+ ]]
}

@test "ps after the container stopped" {
	runc -0 ps test_busybox

	runc -0 kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped

	runc -0 ps test_busybox
}

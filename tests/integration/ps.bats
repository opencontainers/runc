#!/usr/bin/env bats

load helpers

function setup() {
	# ps requires cgroups
	[ $EUID -ne 0 ] && requires rootless_cgroup

	setup_busybox

	# Rootless does not have default cgroup path.
	[ $EUID -ne 0 ] && set_cgroups_path

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	testcontainer test_busybox running
}

function teardown() {
	teardown_bundle
}

@test "ps" {
	run -0 runc ps test_busybox
	[[ "$output" =~ UID\ +PID\ +PPID\ +C\ +STIME\ +TTY\ +TIME\ +CMD+ ]]
	[[ "$output" == *"$(id -un 2>/dev/null)"*[0-9]* ]]
}

@test "ps -f json" {
	run -0 runc ps -f json test_busybox
	[[ "$output" =~ [0-9]+ ]]
}

@test "ps -e -x" {
	run -0 runc ps test_busybox -e -x
	[[ "$output" =~ \ +PID\ +TTY\ +STAT\ +TIME\ +COMMAND+ ]]
	[[ "$output" =~ [0-9]+ ]]
}

@test "ps after the container stopped" {
	run -0 runc ps test_busybox

	run -0 runc kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped

	run -0 runc ps test_busybox
}

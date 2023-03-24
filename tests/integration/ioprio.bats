#!/usr/bin/env bats

load helpers

function setup() {
	setup_debian
}

function teardown() {
	teardown_bundle
}

@test "ioprio_set is applied to process group" {
	# Create a container with a specific I/O priority.
	update_config '.process.ioPriority = {"class": "IOPRIO_CLASS_BE", "priority": 4}'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_ioprio
	[ "$status" -eq 0 ]

	# Check the init process.
	runc exec test_ioprio ionice -p 1
	[ "$status" -eq 0 ]
	[[ "$output" = *'best-effort: prio 4'* ]]

	# Check the process made from the exec command.
	runc exec test_ioprio ionice
	[ "$status" -eq 0 ]

	[[ "$output" = *'best-effort: prio 4'* ]]
}

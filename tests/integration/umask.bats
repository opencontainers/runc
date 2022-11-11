#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "umask" {
	update_config '.process.user += {"umask":63}'

	# run busybox detached
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox grep '^Umask:' "/proc/1/status"
	[ "$status" -eq 0 ]
	# umask 63 decimal = umask 77 octal
	[[ "${output}" == *"77"* ]]

	runc exec test_busybox grep '^Umask:' "/proc/self/status"
	[ "$status" -eq 0 ]
	# umask 63 decimal = umask 77 octal
	[[ "${output}" == *"77"* ]]
}

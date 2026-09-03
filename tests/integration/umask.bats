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

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox grep '^Umask:' "/proc/1/status"
	# umask 63 decimal = umask 77 octal
	assert_output --partial "77"

	run -0 runc exec test_busybox grep '^Umask:' "/proc/self/status"
	# umask 63 decimal = umask 77 octal
	assert_output --partial "77"
}

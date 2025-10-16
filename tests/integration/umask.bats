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

	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	runc -0 exec test_busybox grep '^Umask:' "/proc/1/status"
	# umask 63 decimal = umask 77 octal
	[[ "${output}" == *"77"* ]]

	runc -0 exec test_busybox grep '^Umask:' "/proc/self/status"
	# umask 63 decimal = umask 77 octal
	[[ "${output}" == *"77"* ]]
}

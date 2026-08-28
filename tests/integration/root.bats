#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	ALT_ROOT="$ROOT/alt"
	mkdir -p "$ALT_ROOT/state"
}

function teardown() {
	ROOT=$ALT_ROOT runc delete -f test_dotbox
	unset ALT_ROOT
	teardown_bundle
}

@test "global --root" {
	# run busybox detached using $ALT_ROOT for state
	ROOT=$ALT_ROOT run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_dotbox

	# run busybox detached in default root
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc state test_busybox
	[[ "${output}" == *"running"* ]]

	ROOT=$ALT_ROOT run -0 runc state test_dotbox
	[[ "${output}" == *"running"* ]]

	ROOT=$ALT_ROOT run ! runc state test_busybox

	run ! runc state test_dotbox

	run -0 runc kill test_busybox KILL
	wait_for_container 10 1 test_busybox stopped
	run -0 runc delete test_busybox

	ROOT=$ALT_ROOT run -0 runc kill test_dotbox KILL
	ROOT=$ALT_ROOT wait_for_container 10 1 test_dotbox stopped
	ROOT=$ALT_ROOT run -0 runc delete test_dotbox
}

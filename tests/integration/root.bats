#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	ALT_ROOT="$ROOT/alt"
	mkdir -p "$ALT_ROOT/state"
}

function teardown() {
	ROOT=$ALT_ROOT __runc delete -f test_dotbox
	unset ALT_ROOT
	teardown_bundle
}

@test "global --root" {
	# run busybox detached using $ALT_ROOT for state
	ROOT=$ALT_ROOT runc run -d --console-socket "$CONSOLE_SOCKET" test_dotbox
	[ "$status" -eq 0 ]

	# run busybox detached in default root
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc state test_busybox
	[ "$status" -eq 0 ]
	[[ "${output}" == *"running"* ]]

	ROOT=$ALT_ROOT runc state test_dotbox
	[ "$status" -eq 0 ]
	[[ "${output}" == *"running"* ]]

	ROOT=$ALT_ROOT runc state test_busybox
	[ "$status" -ne 0 ]

	runc state test_dotbox
	[ "$status" -ne 0 ]

	runc kill test_busybox KILL
	[ "$status" -eq 0 ]
	wait_for_container 10 1 test_busybox stopped
	runc delete test_busybox
	[ "$status" -eq 0 ]

	ROOT=$ALT_ROOT runc kill test_dotbox KILL
	[ "$status" -eq 0 ]
	ROOT=$ALT_ROOT wait_for_container 10 1 test_dotbox stopped
	ROOT=$ALT_ROOT runc delete test_dotbox
	[ "$status" -eq 0 ]
}

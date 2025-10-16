#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run detached" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]
	testcontainer test_busybox running
}

@test "runc run detached ({u,g}id != 0)" {
	# cannot start containers as another user in rootless setup without idmap
	[ $EUID -ne 0 ] && requires rootless_idmap

	# replace "uid": 0 with "uid": 1000
	# and do a similar thing for gid.
	update_config ' (.. | select(.uid? == 0)) .uid |= 1000
		| (.. | select(.gid? == 0)) .gid |= 100'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

@test "runc run detached --pid-file" {
	runc run --pid-file pid.txt -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	[ -e pid.txt ]
	[[ "$(cat pid.txt)" == $(__runc state test_busybox | jq '.pid') ]]
}

@test "runc run detached --pid-file with new CWD" {
	bundle="$(pwd)"
	mkdir pid_file
	cd pid_file

	runc run --pid-file pid.txt -d -b "$bundle" --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running

	[ -e pid.txt ]
	[[ "$(cat pid.txt)" == $(__runc state test_busybox | jq '.pid') ]]
}

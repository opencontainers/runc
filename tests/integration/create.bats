#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc create" {
	runc create --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	# start the command
	runc start test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

@test "runc create exec" {
	runc create --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	runc exec test_busybox true
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	# start the command
	runc start test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

@test "runc create --pid-file" {
	runc create --pid-file pid.txt --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	# check pid.txt was generated
	[ -e pid.txt ]

	[[ $(cat pid.txt) = $(__runc state test_busybox | jq '.pid') ]]

	# start the command
	runc start test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

@test "runc create --pid-file with new CWD" {
	bundle="$(pwd)"
	# create pid_file directory as the CWD
	mkdir pid_file
	cd pid_file

	runc create --pid-file pid.txt -b "$bundle" --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox created

	# check pid.txt was generated
	[ -e pid.txt ]

	[[ $(cat pid.txt) = $(__runc state test_busybox | jq '.pid') ]]

	# start the command
	runc start test_busybox
	[ "$status" -eq 0 ]

	testcontainer test_busybox running
}

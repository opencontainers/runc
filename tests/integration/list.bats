#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
	ALT_ROOT="$ROOT/alt"
	mkdir -p "$ALT_ROOT/state"
}

function teardown() {
	ROOT="$ALT_ROOT" teardown_bundle
	unset ALT_ROOT
	teardown_bundle
}

@test "list" {
	bundle=$(pwd)
	ROOT=$ALT_ROOT run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_box1

	ROOT=$ALT_ROOT run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_box2

	ROOT=$ALT_ROOT run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_box3

	ROOT=$ALT_ROOT run -0 runc list
	assert_line --index 0 --regexp 'ID +PID +STATUS +BUNDLE +CREATED+'
	assert_line --index 1 --regexp "test_box1.*[0-9].*running.*$bundle.*[0-9]"
	assert_line --index 2 --regexp "test_box2.*[0-9].*running.*$bundle.*[0-9]"
	assert_line --index 3 --regexp "test_box3.*[0-9].*running.*$bundle.*[0-9]"

	ROOT=$ALT_ROOT run -0 runc list -q
	assert_line --index 0 "test_box1"
	assert_line --index 1 "test_box2"
	assert_line --index 2 "test_box3"

	ROOT=$ALT_ROOT run -0 runc list --format table
	assert_line --index 0 --regexp 'ID +PID +STATUS +BUNDLE +CREATED+'
	assert_line --index 1 --regexp "test_box1.*[0-9].*running.*$bundle.*[0-9]"
	assert_line --index 2 --regexp "test_box2.*[0-9].*running.*$bundle.*[0-9]"
	assert_line --index 3 --regexp "test_box3.*[0-9].*running.*$bundle.*[0-9]"

	ROOT=$ALT_ROOT run -0 runc list --format json
	assert_line --index 0 --regexp '^\[\{"ociVersion":"[0-9]+\.[0-9]+\.[0-9]+","id":"test_box1","pid":[0-9]+,"status":"running","bundle":"[^"]*'"$bundle"'[^"]*","rootfs":"[^"]*","created":[^}]*\}'
	assert_line --index 0 --regexp ',\{"ociVersion":"[0-9]+\.[0-9]+\.[0-9]+","id":"test_box2","pid":[0-9]+,"status":"running","bundle":"[^"]*'"$bundle"'[^"]*","rootfs":"[^"]*","created":[^}]*\}'
	assert_line --index 0 --regexp ',\{"ociVersion":"[0-9]+\.[0-9]+\.[0-9]+","id":"test_box3","pid":[0-9]+,"status":"running","bundle":"[^"]*'"$bundle"'[^"]*","rootfs":"[^"]*","created":[^}]*\}\]$'
}

@test "list with non-existent root fails" {
	ROOT=/non-existent-dir run ! runc list
}

@test "list with default non-existent root succeeds" {
	requires root # rootless auto-creates a directory under $XDG_RUNTIME_DIR.
	test -d /root/runc && skip "requires missing /root/runc"
	ROOT='' run -0 runc list
}

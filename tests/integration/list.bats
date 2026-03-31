#!/usr/bin/env bats

load helpers

# Default root directory for runc (as defined in main.go).
RUNC_DEFAULT_ROOT="/run/runc"

function setup() {
	setup_busybox
	ALT_ROOT="$ROOT/alt"
	mkdir -p "$ALT_ROOT/state"
}

function teardown() {
	# Restore default root if it was backed up by the
	# "non-existent default root" test.
	if [ -d "${RUNC_DEFAULT_ROOT}.bak" ]; then
		rm -rf "$RUNC_DEFAULT_ROOT"
		mv "${RUNC_DEFAULT_ROOT}.bak" "$RUNC_DEFAULT_ROOT"
	fi
	ROOT="$ALT_ROOT" teardown_bundle
	unset ALT_ROOT
	teardown_bundle
}

@test "list" {
	bundle=$(pwd)
	ROOT=$ALT_ROOT runc run -d --console-socket "$CONSOLE_SOCKET" test_box1
	[ "$status" -eq 0 ]

	ROOT=$ALT_ROOT runc run -d --console-socket "$CONSOLE_SOCKET" test_box2
	[ "$status" -eq 0 ]

	ROOT=$ALT_ROOT runc run -d --console-socket "$CONSOLE_SOCKET" test_box3
	[ "$status" -eq 0 ]

	ROOT=$ALT_ROOT runc list
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ ID\ +PID\ +STATUS\ +BUNDLE\ +CREATED+ ]]
	[[ "${lines[1]}" == *"test_box1"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[2]}" == *"test_box2"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[3]}" == *"test_box3"*[0-9]*"running"*$bundle*[0-9]* ]]

	ROOT=$ALT_ROOT runc list -q
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "test_box1" ]
	[ "${lines[1]}" = "test_box2" ]
	[ "${lines[2]}" = "test_box3" ]

	ROOT=$ALT_ROOT runc list --format table
	[ "$status" -eq 0 ]
	[[ ${lines[0]} =~ ID\ +PID\ +STATUS\ +BUNDLE\ +CREATED+ ]]
	[[ "${lines[1]}" == *"test_box1"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[2]}" == *"test_box2"*[0-9]*"running"*$bundle*[0-9]* ]]
	[[ "${lines[3]}" == *"test_box3"*[0-9]*"running"*$bundle*[0-9]* ]]

	ROOT=$ALT_ROOT runc list --format json
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == [\[][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box1\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}]* ]]
	[[ "${lines[0]}" == *[,][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box2\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}]* ]]
	[[ "${lines[0]}" == *[,][\{]"\"ociVersion\""[:]"\""*[0-9][\.]*[0-9][\.]*[0-9]*"\""[,]"\"id\""[:]"\"test_box3\""[,]"\"pid\""[:]*[0-9][,]"\"status\""[:]*"\"running\""[,]"\"bundle\""[:]*$bundle*[,]"\"rootfs\""[:]"\""*"\""[,]"\"created\""[:]*[0-9]*[\}][\]] ]]
}

@test "list with non-existent default root succeeds" {
	# The default root /run/runc is only used when running as real
	# root; rootless uses $XDG_RUNTIME_DIR/runc instead.
	requires root unsafe # Modifies /run/runc.

	# Back up and remove the default root to guarantee it
	# doesn't exist. Restored in teardown.
	[ -d "$RUNC_DEFAULT_ROOT" ] && mv "$RUNC_DEFAULT_ROOT" "${RUNC_DEFAULT_ROOT}.bak"

	# Use ROOT="" so the runc wrapper doesn't pass --root,
	# letting runc use its default root.
	ROOT="" runc list
	[ "$status" -eq 0 ]

	ROOT="" runc list --format json
	[ "$status" -eq 0 ]
	[ "$output" = "null" ]

	ROOT="" runc list -q
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
}

@test "list with empty default root succeeds" {
	# The default root /run/runc is only used when running as real
	# root; rootless uses $XDG_RUNTIME_DIR/runc instead.
	requires root unsafe # Modifies /run/runc.

	# Ensure the default root exists but is empty.
	mkdir -p "$RUNC_DEFAULT_ROOT"

	# Use ROOT="" so the runc wrapper doesn't pass --root,
	# letting runc use its default root.
	ROOT="" runc list
	[ "$status" -eq 0 ]

	ROOT="" runc list --format json
	[ "$status" -eq 0 ]
	[ "$output" = "null" ]

	ROOT="" runc list -q
	[ "$status" -eq 0 ]
	[ "$output" = "" ]
}

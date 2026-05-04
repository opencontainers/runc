#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Create fake rootfs.
	mkdir rootfs/testdir rootfs/testdir2 rootfs/testdir3
	echo "Forbidden information!" >rootfs/testfile

	# add extra masked paths
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testdir2", "/testdir3", "/testfile"]'
}

function teardown() {
	teardown_bundle
}

@test "mask paths [file]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox cat /testfile
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	runc exec test_busybox rm -f /testfile
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_busybox umount /testfile
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Operation not permitted"* ]]
}

@test "mask paths [directory]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox ls /testdir
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	runc exec test_busybox touch /testdir/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_busybox rm -rf /testdir
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_busybox umount /testdir
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Operation not permitted"* ]]
}

@test "mask paths [directories share tmpfs]" {
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	# shellcheck disable=SC2016
	runc exec test_busybox sh -euc '
		set -- $(stat -c %d /testdir /testdir2 /testdir3)
		[ "$1" = "$2" ]
		[ "$2" = "$3" ]
	'
	[ "$status" -eq 0 ]

	runc exec test_busybox touch /testdir2/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]
}

@test "mask paths [directory with read-only rootfs]" {
	update_config '.root.readonly = true'

	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 0 ]

	runc exec test_busybox ls /testdir
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "mask paths [prohibit symlink /proc]" {
	ln -s /symlink rootfs/proc
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must be mounted on ordinary directory"* ]]
}

@test "mask paths [prohibit symlink /sys]" {
	# In rootless containers, /sys is a bind mount not a real sysfs.
	requires root

	ln -s /symlink rootfs/sys
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	# On cgroup v1, this may fail before checking if /sys is a symlink,
	# so we merely check that it fails, and do not check the exact error
	# message like for /proc above.
}

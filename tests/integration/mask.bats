#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Create fake rootfs.
	mkdir rootfs/testdir rootfs/testdir2 rootfs/testdir3
	echo "Forbidden information!" >rootfs/testfile

	# add extra masked paths
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testfile"]'
}

function teardown() {
	teardown_bundle
}

@test "mask paths [file]" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox cat /testfile
	[ -z "$output" ]

	run -1 runc exec test_busybox rm -f /testfile
	assert_output --partial "Read-only file system"

	run -1 runc exec test_busybox umount /testfile
	assert_output --partial "Operation not permitted"
}

@test "mask paths [directory]" {
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox ls /testdir
	[ -z "$output" ]

	run -1 runc exec test_busybox touch /testdir/foo
	assert_output --partial "Read-only file system"

	run -1 runc exec test_busybox rm -rf /testdir
	assert_output --partial "Read-only file system"

	run -1 runc exec test_busybox umount /testdir
	assert_output --partial "Operation not permitted"
}

@test "mask paths [duplicate paths]" {
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testfile"]'
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox sh -c "mount | grep /testdir -c"
	assert_output "1"

	run -0 runc exec test_busybox sh -c "mount | grep /testfile -c"
	assert_output "1"
}

@test "mask paths [prohibit symlink /proc]" {
	ln -s /symlink rootfs/proc
	run -1 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	assert_output --partial "must be mounted on ordinary directory"
}

@test "mask paths [prohibit symlink /sys]" {
	# In rootless containers, /sys is a bind mount not a real sysfs.
	requires root

	ln -s /symlink rootfs/sys
	run -1 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	# On cgroup v1, this may fail before checking if /sys is a symlink,
	# so we merely check that it fails, and do not check the exact error
	# message like for /proc above.
}

@test "mask paths [directories share tmpfs]" {
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir2", "/testdir3"]'
	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	# shellcheck disable=SC2016
	run -0 runc exec test_busybox sh -euc '
		set -- $(stat -c %d /testdir /testdir2 /testdir3)
		[ "$1" = "$2" ]
		[ "$2" = "$3" ]
	'

	run -1 runc exec test_busybox touch /testdir2/foo
	assert_output --partial "Read-only file system"
}

@test "mask paths [directory with read-only rootfs]" {
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir2", "/testdir3"]'
	update_config '.root.readonly = true'

	run -0 runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	run -0 runc exec test_busybox ls /testdir
	[ -z "$output" ]
}

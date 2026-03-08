#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Create fake rootfs.
	mkdir rootfs/testdir
	echo "Forbidden information!" >rootfs/testfile

	# add extra masked paths
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testfile"]'
}

function teardown() {
	teardown_bundle
}

@test "mask paths [file]" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	runc -0 exec test_busybox cat /testfile
	[ -z "$output" ]

	runc -1 exec test_busybox rm -f /testfile
	[[ "${output}" == *"Read-only file system"* ]]

	runc -1 exec test_busybox umount /testfile
	[[ "${output}" == *"Operation not permitted"* ]]
}

@test "mask paths [directory]" {
	runc -0 run -d --console-socket "$CONSOLE_SOCKET" test_busybox

	runc -0 exec test_busybox ls /testdir
	[ -z "$output" ]

	runc -1 exec test_busybox touch /testdir/foo
	[[ "${output}" == *"Read-only file system"* ]]

	runc -1 exec test_busybox rm -rf /testdir
	[[ "${output}" == *"Read-only file system"* ]]

	runc -1 exec test_busybox umount /testdir
	[[ "${output}" == *"Operation not permitted"* ]]
}

@test "mask paths [prohibit symlink /proc]" {
	ln -s /symlink rootfs/proc
	runc -1 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[[ "${output}" == *"must be mounted on ordinary directory"* ]]
}

@test "mask paths [prohibit symlink /sys]" {
	# In rootless containers, /sys is a bind mount not a real sysfs.
	requires root

	ln -s /symlink rootfs/sys
	runc -1 run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	# On cgroup v1, this may fail before checking if /sys is a symlink,
	# so we merely check that it fails, and do not check the exact error
	# message like for /proc above.
}

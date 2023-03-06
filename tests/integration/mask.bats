#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox

	# Create fake rootfs.
	mkdir rootfs/testdir rootfs/testdir2
	echo "Forbidden information!" >rootfs/testfile
	echo "Forbidden information! (in a nested dir)" >rootfs/testdir2/file2

	# add extra masked paths
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testfile", "/testdir2/testfile2"]'
}

function teardown() {
	teardown_bundle
}

@test "mask paths [file]" {
	# run busybox detached
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
	# run busybox detached
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

@test "mask paths [prohibit symlink /proc]" {
	ln -s /symlink rootfs/proc
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must not be a symlink"* ]]
}

@test "mask paths [prohibit symlink /sys]" {
	ln -s /symlink rootfs/sys
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must not be a symlink"* ]]
}

@test "mask paths [prohibit symlink /testdir]" {
	rmdir rootfs/testdir
	ln -s /symlink rootfs/testdir
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must not be a symlink"* ]]
}

@test "mask paths [prohibit symlink /testfile]" {
	rm -f rootfs/testfile
	ln -s /symlink rootfs/testfile
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must not be a symlink"* ]]
}

@test "mask paths [prohibit symlink /testdir2 (parent of /testdir2/testfile2)]" {
	rm -rf rootfs/testdir2
	ln -s /symlink rootfs/testdir2
	runc run -d --console-socket "$CONSOLE_SOCKET" test_busybox
	[ "$status" -eq 1 ]
	[[ "${output}" == *"must not be a symlink"* ]]
}

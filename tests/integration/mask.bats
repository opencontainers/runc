#!/usr/bin/env bats

load helpers

function setup() {
	teardown_container
	setup_container

	# Create fake rootfs.
	mkdir rootfs/testdir
	echo "Forbidden information!" > rootfs/testfile

	# add extra masked paths
	update_config '(.. | select(.maskedPaths? != null)) .maskedPaths += ["/testdir", "/testfile"]'
}

function teardown() {
	teardown_container
}

@test "mask paths [file]" {
	runc run -d --console-socket $CONSOLE_SOCKET test_container
	[ "$status" -eq 0 ]

	runc exec test_container cat /testfile
	[ "$status" -eq 0 ]
	[[ "${output}" == "" ]]

	runc exec test_container rm -f /testfile
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_container umount /testfile
	[ "$status" -eq 32 ]
	[[ "${output}" == *"must be superuser to unmount"* ]]
}

@test "mask paths [directory]" {
	runc run -d --console-socket $CONSOLE_SOCKET test_container
	[ "$status" -eq 0 ]

	runc exec test_container ls /testdir
	[ "$status" -eq 0 ]
	[[ "${output}" == "" ]]

	runc exec test_container touch /testdir/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_container rm -rf /testdir
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_container umount /testdir
	[ "$status" -eq 32 ]
	[[ "${output}" == *"must be superuser to unmount"* ]]
}

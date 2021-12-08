#!/usr/bin/env bats

load helpers

TESTVOLUME="${BATS_RUN_TMPDIR}/mounts_recursive"

function setup_volume() {
	# requires root (in the current user namespace) to mount tmpfs outside runc
	requires root

	mkdir -p "${TESTVOLUME}"
	mount -t tmpfs none "${TESTVOLUME}"
	echo "foo" >"${TESTVOLUME}/foo"

	mkdir "${TESTVOLUME}/subvol"
	mount -t tmpfs none "${TESTVOLUME}/subvol"
	echo "bar" >"${TESTVOLUME}/subvol/bar"
}

function teardown_volume() {
	umount -R "${TESTVOLUME}"
}

function setup() {
	setup_volume
	setup_busybox
}

function teardown() {
	teardown_volume
	teardown_bundle
}

@test "runc run [rbind,ro mount is read-only but not recursively]" {
	update_config ".mounts += [{source: \"${TESTVOLUME}\" , destination: \"/mnt\", options: [\"rbind\",\"ro\"]}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_rbind_ro
	[ "$status" -eq 0 ]

	runc exec test_rbind_ro touch /mnt/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_rbind_ro touch /mnt/subvol/bar
	[ "$status" -eq 0 ]
}

@test "runc run [rbind,rro mount is recursively read-only]" {
	requires_kernel 5.12
	update_config ".mounts += [{source: \"${TESTVOLUME}\" , destination: \"/mnt\", options: [\"rbind\",\"rro\"]}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_rbind_rro
	[ "$status" -eq 0 ]

	runc exec test_rbind_rro touch /mnt/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_rbind_rro touch /mnt/subvol/bar
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]
}

@test "runc run [rbind,ro,rro mount is recursively read-only too]" {
	requires_kernel 5.12
	update_config ".mounts += [{source: \"${TESTVOLUME}\" , destination: \"/mnt\", options: [\"rbind\",\"ro\",\"rro\"]}]"

	runc run -d --console-socket "$CONSOLE_SOCKET" test_rbind_ro_rro
	[ "$status" -eq 0 ]

	runc exec test_rbind_ro_rro touch /mnt/foo
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]

	runc exec test_rbind_ro_rro touch /mnt/subvol/bar
	[ "$status" -eq 1 ]
	[[ "${output}" == *"Read-only file system"* ]]
}

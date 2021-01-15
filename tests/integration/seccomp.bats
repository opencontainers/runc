#!/usr/bin/env bats

load helpers

function setup() {
	teardown_busybox
	setup_busybox
}

function teardown() {
	teardown_busybox
}

@test "runc run [seccomp -ENOSYS handling]" {
	TEST_NAME="seccomp_syscall_test1"

	# Compile the test binary and update the config to run it.
	gcc -static -o rootfs/seccomp_test "${TESTDATA}/${TEST_NAME}.c"
	update_config ".linux.seccomp = $(<"${TESTDATA}/${TEST_NAME}.json")"
	update_config '.process.args = ["/seccomp_test"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

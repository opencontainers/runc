#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
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

@test "runc run [seccomp defaultErrnoRet=ENXIO]" {
	TEST_NAME="seccomp_syscall_test2"

	# Compile the test binary and update the config to run it.
	gcc -static -o rootfs/seccomp_test2 "${TESTDATA}/${TEST_NAME}.c"
	update_config ".linux.seccomp = $(<"${TESTDATA}/${TEST_NAME}.json")"
	update_config '.process.args = ["/seccomp_test2"]'

	runc run test_busybox
	[ "$status" -eq 0 ]
}

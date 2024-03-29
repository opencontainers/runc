#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

@test "runc run with RLIMIT_NOFILE" {
	update_config '.process.args = ["/bin/sh", "-c", "ulimit -n"]'
	update_config '.process.capabilities.bounding = ["CAP_SYS_RESOURCE"]'
	update_config '.process.rlimits = [{"type": "RLIMIT_NOFILE", "hard": 10000, "soft": 10000}]'

	runc run test_hello
	[ "$status" -eq 0 ]

	[[ "${output}" == "10000" ]]
}

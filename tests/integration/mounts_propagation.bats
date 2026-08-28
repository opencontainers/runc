#!/usr/bin/env bats

load helpers

function setup() {
	requires root
	setup_debian
}

function teardown() {
	teardown_bundle
}

@test "runc run [rootfsPropagation shared]" {
	update_config ' .linux.rootfsPropagation = "shared" '

	update_config ' .process.args = ["findmnt", "--noheadings", "-o", "PROPAGATION", "/"] '

	run -0 runc run test_shared_rootfs
	[ "$output" = "shared" ]
}
